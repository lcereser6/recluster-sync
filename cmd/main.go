/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package main

import (
	"crypto/tls"
	"flag"
	"os"
	"path/filepath"

	_ "k8s.io/client-go/plugin/pkg/client/auth" // auth providers

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	reclusterv1alpha1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
	"github.com/lcereser6/recluster-sync/internal/controller"
	"github.com/lcereser6/recluster-sync/internal/state" // *** NEW ***
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(reclusterv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	log := ctrl.Log.WithName("setup 35")

	/* ------------------------- CLI flags -------------------------- */

	var (
		metricsAddr                                      string
		metricsCertPath, metricsCertName, metricsCertKey string
		webhookCertPath, webhookCertName, webhookCertKey string
		enableLeaderElection, secureMetrics, enableHTTP2 bool
		probeAddr                                        string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0",
		"The address the metrics endpoint binds to (':8080', ':8443', or '0' to disable).")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081",
		"The address the health/ready probe binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"Serve metrics via HTTPS. Use --metrics-secure=false for HTTP.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"Enable HTTP/2 for webhook & metrics servers (disabled by default).")

	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "Directory that contains webhook cert.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "Webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "Webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "", "Directory that contains metrics cert.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "Metrics certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "Metrics key file.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	/* ----------------- TLS helper to disable HTTP/2 ---------------- */

	var tlsOpts []func(*tls.Config)
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			log.Info("HTTP/2 disabled for webhook & metrics servers")
			c.NextProtos = []string{"http/1.1"}
		})
	}

	/* ---------------- Cert watchers (optional) -------------------- */

	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher
	if webhookCertPath != "" {
		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			log.Error(err, "unable to initialize webhook certificate watcher")
			os.Exit(1)
		}
		tlsOpts = append(tlsOpts, func(cfg *tls.Config) {
			cfg.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{TLSOpts: tlsOpts})

	metricsSrvOpts := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}
	if secureMetrics {
		metricsSrvOpts.FilterProvider = filters.WithAuthenticationAndAuthorization
		if metricsCertPath != "" {
			var err error
			metricsCertWatcher, err = certwatcher.New(
				filepath.Join(metricsCertPath, metricsCertName),
				filepath.Join(metricsCertPath, metricsCertKey),
			)
			if err != nil {
				log.Error(err, "unable to init metrics cert watcher")
				os.Exit(1)
			}
			metricsSrvOpts.TLSOpts = append(metricsSrvOpts.TLSOpts,
				func(c *tls.Config) { c.GetCertificate = metricsCertWatcher.GetCertificate })
		}
	}

	/* ----------------------- Manager ------------------------------------ */

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsSrvOpts,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "0274e017.recluster.com",
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	/* ---------------- ----- State cache (Task 4) ------------------------- */

	st, err := state.New(ctrl.GetConfigOrDie())
	if err != nil {
		log.Error(err, "unable to create live state cache")
		os.Exit(1)
	}
	// Add as Runnable → manager will call Start(ctx)
	if err := mgr.Add(st); err != nil {
		log.Error(err, "unable to add state cache to manager")
		os.Exit(1)
	}

	/* ---------------------- Controllers --------------------------------- */

	if err = (&controller.RcnodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		State:  st, // *** NEW – inject live cache ***
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create Rcnode controller")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	/* -------------- Add cert watchers to manager (optional) ------------- */

	if metricsCertWatcher != nil {
		if err := mgr.Add(metricsCertWatcher); err != nil {
			log.Error(err, "unable to add metrics cert watcher")
			os.Exit(1)
		}
	}
	if webhookCertWatcher != nil {
		if err := mgr.Add(webhookCertWatcher); err != nil {
			log.Error(err, "unable to add webhook cert watcher")
			os.Exit(1)
		}
	}

	/* ---------------- health / ready probes ----------------------------- */

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	/* --------------------------- GO! ------------------------------------ */

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

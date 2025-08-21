// cmd/main.go
//
// Entry-point for the *recluster-sync* controller-manager.
//
// The binary is completely self-contained – it will:
//
//   • parse CLI flags (metrics, probes, TLS, leader-election …)
//   • wire an optional certificate hot-reloader for both webhook & metrics
//   • disable HTTP/2 unless the operator explicitly enables it
//   • spin-up controller-runtime’s manager with the custom Scheme
//   • start the live *State* cache (internal/state)
//   • register the RcNode reconciler (and any future controllers)
//   • expose /healthz & /readyz endpoints
//
// Every important step is logged with a **single, unambiguous** message so that
// `kubectl logs` (or a structured log pipeline) can show a linear story of the
// process-lifecycle.

package main

import (
	"crypto/tls"
	"flag"
	"os"
	"path/filepath"
	"strconv"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // enable e.g. GCP, OIDC, Azure …

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
	"github.com/lcereser6/recluster-sync/internal/backend"
	"github.com/lcereser6/recluster-sync/internal/controller"
	"github.com/lcereser6/recluster-sync/internal/state"
	wh "github.com/lcereser6/recluster-sync/internal/webhook"
)

/* -------------------------------------------------------------------------- */
/*                               scheme wiring                                */
/* -------------------------------------------------------------------------- */

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))    // core + apps, rbac …
	utilruntime.Must(reclusterv1alpha1.AddToScheme(scheme)) // our CRDs
	// +kubebuilder:scaffold:scheme                                // keep hook
}

/* -------------------------------------------------------------------------- */

func main() {
	/* ======================= CLI flags & logger ======================== */

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)

	var (
		metricsAddr   string
		probeAddr     string
		enableLeader  bool
		secureMetrics bool
		enableHTTP2   bool

		webhookCertPath, webhookCertName, webhookCertKey string
		metricsCertPath, metricsCertName, metricsCertKey string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "':8080', ':8443' or '0' (disabled)")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "liveness / readiness probe address")

	flag.BoolVar(&enableLeader, "leader-elect", false, "Elect a single active manager")
	flag.BoolVar(&secureMetrics, "metrics-secure", true, "Serve metrics over HTTPS")
	flag.BoolVar(&enableHTTP2, "enable-http2", false, "Opt-in to HTTP/2 (off by default)")

	// optional TLS cert locations
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "Dir containing webhook cert/key")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "Webhook cert filename")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "Webhook key filename")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "", "Dir containing metrics cert/key")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "Metrics cert filename")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "Metrics key filename")

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("bootstrap")
	log.Info("recluster-sync controller-manager starting v 0.6")

	log.Info("parsed CLI flags",
		"metrics", metricsAddr, "probe", probeAddr,
		"secureMetrics", secureMetrics, "http2", enableHTTP2,
		"leaderElection", enableLeader)

	/* ======================= HTTP/2 hardening ========================= */

	var tlsOpts []func(*tls.Config)
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			c.NextProtos = []string{"http/1.1"}
		})
		log.Info("HTTP/2 is disabled to protect against Rapid-Reset class CVEs")
	}

	/* ================= Optional cert hot-reloaders ==================== */

	var webhookWatcher, metricsWatcher *certwatcher.CertWatcher
	if webhookCertPath != "" {
		w, err := certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey))
		if err != nil {
			log.Error(err, "cannot start webhook cert-watcher")
			os.Exit(1)
		}
		webhookWatcher = w
		tlsOpts = append(tlsOpts, func(cfg *tls.Config) { cfg.GetCertificate = w.GetCertificate })
		log.Info("webhook certificate watcher initialised")
	}

	webhookSrv := webhook.NewServer(webhook.Options{
		Port:    9443,
		CertDir: "/tmp/k8s-webhook-server/serving-certs",
	})

	metricsOpts := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}
	if secureMetrics {
		metricsOpts.FilterProvider = filters.WithAuthenticationAndAuthorization
		if metricsCertPath != "" {
			w, err := certwatcher.New(
				filepath.Join(metricsCertPath, metricsCertName),
				filepath.Join(metricsCertPath, metricsCertKey))
			if err != nil {
				log.Error(err, "cannot start metrics cert-watcher")
				os.Exit(1)
			}
			metricsWatcher = w
			metricsOpts.TLSOpts = append(metricsOpts.TLSOpts, func(cfg *tls.Config) {
				cfg.GetCertificate = w.GetCertificate
			})
			log.Info("metrics certificate watcher initialised")
		}
	}

	/* ======================== controller-manager ====================== */

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		LeaderElection:         enableLeader,
		LeaderElectionID:       "0274e017.recluster.com",
		Metrics:                metricsOpts,
		WebhookServer:          webhookSrv,
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		log.Error(err, "cannot build controller-manager")
		os.Exit(1)
	}
	log.Info("controller-manager created")
	webhookSrv.Register("/mutate-v1-pod", &webhook.Admission{Handler: &wh.GateInjector{}})

	/* ======================= live-state cache ========================= */

	st, err := state.New(ctrl.GetConfigOrDie())
	if err != nil {
		log.Error(err, "cannot initialise live state cache")
		os.Exit(1)
	}
	if err := mgr.Add(st); err != nil {
		log.Error(err, "cannot add state cache to manager")
		os.Exit(1)
	}
	log.Info("live state cache registered")
	// 1. Pick backend from env injected by Helm
	mode := os.Getenv("RECLUSTER_BACKEND_MODE") // kwok | prod | test
	be, err := backend.New(mode, kubernetes.NewForConfigOrDie(ctrl.GetConfigOrDie()))
	if err != nil {
		log.Error(err, "invalid backend mode")
		os.Exit(1)
	}

	// 2. RcNode controller gets the backend
	if err := controller.NewRcNodeReconciler(mgr, be).SetupWithManager(mgr); err != nil {
		log.Error(err, "cannot set up RcNode controller")
		os.Exit(1)
	}

	// 3. (unchanged) add PodReconciler and Planner
	if err := controller.NewPodReconciler(mgr).SetupWithManager(mgr); err != nil {
		log.Error(err, "cannot set up Pod controller")
		os.Exit(1)
	}

	//GET cooldown from env, default to 5 seconds
	cooldown := os.Getenv("RECLUSTER_PLANNER_COOLDOWN")
	if cooldown == "" {
		cooldown = "5" // default cooldown
	}
	cooldownInt, err := strconv.Atoi(cooldown)

	log.Info("cooldown for planner set to", "seconds", cooldownInt)

	// if err := mgr.Add(graph.NewPlanner(mgr, st, cooldownInt)); err != nil {
	// 	log.Error(err, "cannot add planner runnable")
	// 	os.Exit(1)
	// }
	/* =================== extra runnables (certs) ===================== */

	if metricsWatcher != nil {
		if err := mgr.Add(metricsWatcher); err != nil {
			log.Error(err, "adding metrics cert-watcher failed")
			os.Exit(1)
		}
		log.Info("metrics cert-watcher added to manager")
	}
	if webhookWatcher != nil {
		if err := mgr.Add(webhookWatcher); err != nil {
			log.Error(err, "adding webhook cert-watcher failed")
			os.Exit(1)
		}
		log.Info("webhook cert-watcher added to manager")
	}

	/* ===================== probes & startup ========================== */

	utilruntime.Must(mgr.AddHealthzCheck("healthz", healthz.Ping))
	utilruntime.Must(mgr.AddReadyzCheck("readyz", healthz.Ping))
	log.Info("health & readiness probes registered")

	log.Info("starting controller-manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "manager runtime exited")
		os.Exit(1)
	}
}

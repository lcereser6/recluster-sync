// rcnode_types.go
/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient

// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RcNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RcNodeSpec   `json:"spec,omitempty"`
	Status RcNodeStatus `json:"status,omitempty"`
}

/* -------------------------------------------------------------------------- */
/*                           Desired‑state (Spec)                             */
/* -------------------------------------------------------------------------- */

type RcNodeSpec struct {
	/* ---------- identity & membership ---------- */
	Roles       []NodeRole       `json:"roles,omitempty"`
	Permissions []NodePermission `json:"permissions,omitempty"`
	NodePool    string           `json:"nodePool,omitempty"` // pool name

	/* ------------------ hardware ---------------- */
	Address string                `json:"address"`
	CPU     RcNodeCPUSpec         `json:"cpu"`
	Memory  int64                 `json:"memoryBytes"` // bytes to avoid GiB rounding issues
	Storage []RcNodeStorageSpec   `json:"storages,omitempty"`
	Network []RcNodeInterfaceSpec `json:"interfaces,omitempty"`

	/* ---------- power‑consumption model ---------- */
	MinPowerConsumption            int                   `json:"minPowerConsumption,omitempty"`
	MaxEfficiencyPowerConsumption  *int                  `json:"maxEfficiencyPowerConsumption,omitempty"`
	MinPerformancePowerConsumption *int                  `json:"minPerformancePowerConsumption,omitempty"`
	MaxPowerConsumption            int                   `json:"maxPowerConsumption,omitempty"`
	PowerCurve                     *RcNodePowerCurveSpec `json:"powerCurve,omitempty"`

	/* ---------- lifecycle control ---------- */
	BootSeconds  int    `json:"bootSeconds,omitempty"`  // 0 = powered off
	DesiredState string `json:"desiredState,omitempty"` // "Running" | "Stopped" | etc.
}

/* -------------------------------------------------------------------------- */
/*                            Observed‑state (Status)                         */
/* -------------------------------------------------------------------------- */

type RcNodeStatus struct {
	State               NodeStatus   `json:"state,omitempty"`
	Reason              string       `json:"reason,omitempty"`
	Message             string       `json:"message,omitempty"`
	LastHeartbeat       *metav1.Time `json:"lastHeartbeat,omitempty"`
	LastTransition      *metav1.Time `json:"lastTransition,omitempty"`
	NodePoolAssigned    bool         `json:"nodePoolAssigned,omitempty"`
	UtilizationMilliCPU int          `json:"utilizationMilliCPU,omitempty"` // scheduled requests sum
	UtilizationPct      float64      `json:"utilizationPct,omitempty"`      // derived percentage (0–100)
	PredictedPowerWatts int          `json:"predictedPowerWatts,omitempty"` // interpolated from curve
	ObservedPowerWatts  *int         `json:"observedPowerWatts,omitempty"`  // optional real‑time reading
}

/* -------------------------------------------------------------------------- */
/*                         Power‑consumption modelling                        */
/* -------------------------------------------------------------------------- */

type RcNodePowerCurvePoint struct {
	LoadPct    int `json:"loadPct"`
	PowerWatts int `json:"powerWatts"`
}

type RcNodePowerCurveSpec struct {
	Points []RcNodePowerCurvePoint `json:"points"`
}

/* -------------------------------------------------------------------------- */
/*                              Sub‑resources                                 */
/* -------------------------------------------------------------------------- */

type RcNodeCPUSpec struct {
	Architecture         CpuArchitecture `json:"architecture"`
	Vendor               CpuVendor       `json:"vendor"`
	Family               int             `json:"family"`
	Model                int             `json:"model"`
	Name                 string          `json:"name"`
	Cores                int             `json:"cores"`
	Flags                []string        `json:"flags,omitempty"`
	CacheL1d             int             `json:"cacheL1d,omitempty"`
	CacheL1i             int             `json:"cacheL1i,omitempty"`
	CacheL2              int             `json:"cacheL2,omitempty"`
	CacheL3              int             `json:"cacheL3,omitempty"`
	Vulnerabilities      []string        `json:"vulnerabilities,omitempty"`
	SingleThreadScore    int             `json:"singleThreadScore,omitempty"`
	MultiThreadScore     int             `json:"multiThreadScore,omitempty"`
	EfficiencyThreshold  *int            `json:"efficiencyThreshold,omitempty"`
	PerformanceThreshold *int            `json:"performanceThreshold,omitempty"`
}

// RcNodeStorageSpec describes an attached storage device.

type RcNodeStorageSpec struct {
	Name string `json:"name"`
	// Size in bytes (aligning with Prisma’s BigInt)
	Size int64 `json:"size"`
}

// RcNodeInterfaceSpec describes a network interface.

type RcNodeInterfaceSpec struct {
	Name    string    `json:"name"`
	Address string    `json:"address"`
	Speed   int64     `json:"speed,omitempty"` // bits per second
	WoL     []WoLFlag `json:"wol,omitempty"`
}

/* -------------------------------------------------------------------------- */
/*                              Enumerations                                  */
/* -------------------------------------------------------------------------- */

type NodeRole string

const (
	NodeRoleReclusterController NodeRole = "RECLUSTER_CONTROLLER"
	NodeRoleK8sController       NodeRole = "K8S_CONTROLLER"
	NodeRoleK8sWorker           NodeRole = "K8S_WORKER"
)

type NodePermission string

const (
	NodePermissionUnknown NodePermission = "UNKNOWN"
)

type NodeStatus string

const (
	NodeStatusActive         NodeStatus = "ACTIVE"
	NodeStatusActiveReady    NodeStatus = "ACTIVE_READY"
	NodeStatusActiveNotReady NodeStatus = "ACTIVE_NOT_READY"
	NodeStatusActiveDeleting NodeStatus = "ACTIVE_DELETING"
	NodeStatusBooting        NodeStatus = "BOOTING"
	NodeStatusInactive       NodeStatus = "INACTIVE"
	NodeStatusUnknown        NodeStatus = "UNKNOWN"
)

type CpuArchitecture string

const (
	CpuArchAMD64 CpuArchitecture = "AMD64"
	CpuArchARM64 CpuArchitecture = "ARM64"
)

type CpuVendor string

const (
	CpuVendorAMD   CpuVendor = "AMD"
	CpuVendorIntel CpuVendor = "INTEL"
)

type WoLFlag string

const (
	WoLFlagA WoLFlag = "a"
	WoLFlagB WoLFlag = "b"
	WoLFlagG WoLFlag = "g"
	WoLFlagM WoLFlag = "m"
	WoLFlagP WoLFlag = "p"
	WoLFlagS WoLFlag = "s"
	WoLFlagU WoLFlag = "u"
)

/* -------------------------------------------------------------------------- */
/*                        List‑type scaffolding                               */
/* -------------------------------------------------------------------------- */

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RcNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RcNode `json:"items"`
}

/* -------------------------------------------------------------------------- */
/*                               Registration                                 */
/* -------------------------------------------------------------------------- */

func init() {
	SchemeBuilder.Register(&RcNode{}, &RcNodeList{})
}

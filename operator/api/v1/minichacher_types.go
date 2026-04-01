package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MiniChacheRSpec defines the desired state of MiniChacheR
type MiniChacheRSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// Replicas is the desired number of cache nodes in the cluster.
	// This should typically be an odd number for Raft quorum (e.g., 3, 5).
	Replicas *int32 `json:"replicas"`

	// Image is the container image used to run each cache node.
	// The operator will deploy pods using this image.
	Image string `json:"image"`
}

// MiniChacheRStatus defines the observed state of MiniChacheR.
type MiniChacheRStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the MiniChacheR resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Number of cache pods that have passed readiness checks and are serving traffic.
	ReadyReplicas int32 `json:"readyReplicas"`

	// Names of all pods currently part of the MiniCacheR cluster.
	Nodes []string `json:"nodes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MiniChacheR is the Schema for the minichachers API
type MiniChacheR struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of MiniChacheR
	// +required
	Spec MiniChacheRSpec `json:"spec"`

	// status defines the observed state of MiniChacheR
	// +optional
	Status MiniChacheRStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MiniChacheRList contains a list of MiniChacheR
type MiniChacheRList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []MiniChacheR `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MiniChacheR{}, &MiniChacheRList{})
}

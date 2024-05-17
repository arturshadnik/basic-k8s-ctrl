/*
Copyright 2024 arturshadnik.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterScanSpec defines the desired state of ClusterScan
type ClusterScanSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	JobType                 string            `json:"jobType,omitempty"`
	Image                   string            `json:"image,omitempty"`
	Command                 []string          `json:"command,omitempty"`
	Args                    []string          `json:"args,omitempty"`
	Schedule                string            `json:"schedule,omitempty"`
	OneOff                  bool              `json:"oneOff,omitempty"`
	ScanConfig              map[string]string `json:"scanConfig,omitempty"`
	StartingDeadlineSeconds int               `json:"startingDeadlineSeconds,omitempty"`
	RestartPolicy           string            `json:"restartPolicy"`
}

// ClusterScanStatus defines the observed state of ClusterScan
type ClusterScanStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase                string           `json:"phase,omitempty"`
	StartTime            metav1.Time      `json:"startTime,omitempty"`
	CompletionTime       metav1.Time      `json:"completionTime,omitempty"`
	Active               int              `json:"active,omitempty"`
	Succeeded            int              `json:"succeeded,omitempty"`
	Failed               int              `json:"failed,omitempty"`
	LastExecutionDetails ExecutionDetails `json:"executionDetails,omitempty"`
	NextScheduledTime    metav1.Time      `json:"nextScheduledTime,omitempty"`
	ErrorMessage         string           `json:"errorMessage,omitempty"`
	Conditions           []Conditions     `json:"Conditions,omitempty"`
}

type ExecutionDetails struct {
	StartTime      metav1.Time `json:"startTime,omitempty"`
	CompletionTime metav1.Time `json:"completionTime,omitempty"`
	Result         string      `json:"result,omitempty"`
}

type Conditions struct {
	Type               string      `json:"type,omitempty"`
	Status             string      `json:"status,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterScan is the Schema for the clusterscans API
type ClusterScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterScanSpec   `json:"spec,omitempty"`
	Status ClusterScanStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterScanList contains a list of ClusterScan
type ClusterScanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterScan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterScan{}, &ClusterScanList{})
}

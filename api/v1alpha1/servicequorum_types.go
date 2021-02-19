/*
Copyright 2021 Guilhem Lettron.

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
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceQuorumSpec defines the desired state of ServiceQuorum
type ServiceQuorumSpec struct {
	Deployment string           `json:"deployment,omitempty"`
	Template   core.ServiceSpec `json:"template"`
}

// ServiceQuorumStatus defines the observed state of ServiceQuorum
type ServiceQuorumStatus struct {
	ReplicaSets       []string `json:"replicaSets,omitempty"`
	CurrentReplicaSet string   `json:"currentReplicatset"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=servicesquorum

// ServiceQuorum is the Schema for the servicesquorum API
type ServiceQuorum struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceQuorumSpec   `json:"spec,omitempty"`
	Status ServiceQuorumStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceQuorumList contains a list of ServiceQuorum
type ServiceQuorumList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceQuorum `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceQuorum{}, &ServiceQuorumList{})
}

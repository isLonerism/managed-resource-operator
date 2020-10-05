/*
Copyright 2020 Vladislav Poberezhny.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"operator/pkg/utils"
)

// ManagedResourceBindingItem is a kubernetes object and its permission verbs
type ManagedResourceBindingItem struct {
	Object utils.ManagedResourceStruct `json:"object"`

	// +kubebuilder:validation:MinItems=1
	Verbs []utils.Verb `json:"verbs"`
}

// ManagedResourceBindingSpec defines the desired state of ManagedResourceBinding
type ManagedResourceBindingSpec struct {

	// +kubebuilder:validation:MinItems=1
	Items []ManagedResourceBindingItem `json:"items"`

	// +kubebuilder:validation:MinItems=1
	Namespaces []utils.Namespace `json:"namespaces"`
}

// ManagedResourceBindingStatus defines the observed state of ManagedResourceBinding
type ManagedResourceBindingStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mrb,scope=Cluster

// ManagedResourceBinding is the Schema for the managedresourcebindings API
type ManagedResourceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedResourceBindingSpec   `json:"spec,omitempty"`
	Status ManagedResourceBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedResourceBindingList contains a list of ManagedResourceBinding
type ManagedResourceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedResourceBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagedResourceBinding{}, &ManagedResourceBindingList{})
}

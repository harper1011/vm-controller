/*
Copyright 2024.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// UpCloudVMSpec defines the desired state of UpCloudVM
type UpCloudVMSpec struct {
	CPU      int    `json:"cpu"`
	Memory   int    `json:"memory"`
	Storage  int    `json:"storage"`
	Zone     string `json:"zone"`
	Plan     string `json:"plan"`
	Template string `json:"template"`
}

// UpCloudVMStatus defines the observed state of UpCloudVM
type UpCloudVMStatus struct {
	VMID      string `json:"vmID,omitempty"`
	State     string `json:"state,omitempty"`
	IPAddress string `json:"ipAddress,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// UpCloudVM is the Schema for the upcloudvms API
type UpCloudVM struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpCloudVMSpec   `json:"spec,omitempty"`
	Status UpCloudVMStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UpCloudVMList contains a list of UpCloudVM
type UpCloudVMList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UpCloudVM `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UpCloudVM{}, &UpCloudVMList{})
}

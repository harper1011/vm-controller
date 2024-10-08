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
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpCloudVMSpec defines the desired state of UpCloudVM
type UpCloudVMSpec struct {
	CPU             int                `json:"cpu"`
	Memory          int                `json:"memory"`
	StorageSize     int                `json:"storagesize"`
	Zone            string             `json:"zone"`
	Plan            string             `json:"plan"`
	TimeZone        string             `json:"timezone"`
	StorageTemplate string             `json:"storagetemplate"`
	LoginUser       *request.LoginUser `json:"login_user,omitempty"`
	UserData        string             `json:"user_data,omitempty"`

	//Comment for further improvement:
	// - What is the size limit for this UserData? if it has the same size limit as OpenStack, then we might need to encode it with base64
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

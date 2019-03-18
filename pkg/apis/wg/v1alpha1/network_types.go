/*
Copyright 2019 The kubewg Authors.

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

// NetworkSpec defines the desired state of Network
type NetworkSpec struct {
	// Subnet is the subnet that encompassing this Wireguard network.
	// Peer addresses will be automatically assigned out of this subnet.
	Subnet string `json:"subnet"`

	// Rules for allocating IP addresses to peers
	// +optional
	Allocations []AllocationRule `json:"allocations,omitempty"`
}

type AllocationRule struct {
	// Address is a designated static address for this peer.
	// +optional
	Address *string `json:"address,omitempty"`

	// Selector matches peers that should be allocated an address using this
	// allocation rule.
	// If not set, this rule will match all peers and act as the default IP
	// allocation mechanism for the Network.
	// +optional
	Selector *PeerSelector `json:"selector,omitempty"`
}

// NetworkStatus defines the observed state of Network
type NetworkStatus struct {
	// The list of assigned IP addresses for peers
	// +optional
	Allocations []IPAssignment `json:"allocations,omitempty"`
}

type IPAssignment struct {
	// The name of the Peer that has been allocated an address
	Name string `json:"name"`

	// The allocated IP address
	Address string `json:"address"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Network is the Schema for the networks API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec,omitempty"`
	Status NetworkStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkList contains a list of Network
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}

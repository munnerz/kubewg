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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PeerSpec defines the desired state of Peer
type PeerSpec struct {
	// PublicKey that should be used to authenticate traffic from this peer
	PublicKey string `json:"publicKey"`

	// Endpoint is an optional endpoint that should be used to connect to this
	// peer.
	// If not specified, other peers in the network will accept connections
	// from this peer, but no direct connection will be made *to* the peer.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// PeerStatus defines the observed state of Peer
type PeerStatus struct {
	// Address is the allocated IP address of this peer within the VPN network
	// +optional
	Address string `json:"address,omitempty"`

	// Network is the name of the VPN network that this peer belongs to
	// +optional
	Network string `json:"network,omitempty"`

	// Peers is a computed list of peers that should be configured on this
	// peer's wireguard interface.
	Peers []PeerConfiguration `json:"peers,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Peer is the Schema for the peers API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Peer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PeerSpec   `json:"spec,omitempty"`
	Status PeerStatus `json:"status,omitempty"`
}

type PeerConfiguration struct {
	// Name is the peer's name, as stored in its metadata
	Name string `json:"name"`

	// PublicKey is the public key that should be used to authenticate traffic
	// from this peer
	PublicKey string `json:"publicKey"`

	// Endpoint is the optional endpoint address to connect to in order to
	// establish a secure Wireguard link.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// AllowedIPs is a list of IP addresses that should be allowed as the src
	// parameter on IP packets coming from this peer.
	// This also acts as a loose routing table, where subnet named here will be
	// routed via this peer.
	// +optional
	AllowedIPs []string `json:"allowedIPs,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PeerList contains a list of Peer
type PeerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Peer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Peer{}, &PeerList{})
}

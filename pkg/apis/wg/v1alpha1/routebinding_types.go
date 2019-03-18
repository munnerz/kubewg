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

// RouteBindingSpec defines the desired state of RouteBinding
type RouteBindingSpec struct {
	// Routes is a list of subnets that should be routed via peers matching the
	// given selector.
	// If a peer matches the selector, all routes named here will be configured
	// via that peer.
	// If multiple peers match, then multiple routes will be created and it is
	// up to the systems routing table to decide which route to use.
	Routes []string `json:"routes,omitempty"`

	// Selector selects peers to route traffic to
	Selector PeerSelector `json:"selector"`

	// Network is the name of the network this route applies within.
	Network string `json:"network"`
}

// RouteBindingStatus defines the observed state of RouteBinding
type RouteBindingStatus struct {
}

type PeerSelector struct {
	// Names is a list of peer names that match this selector.
	// If multiple names are provided, a Peer 'matches' if its name is
	// contained within this slice.
	Names []string `json:"names,omitempty"`

	// MatchLabels can be used to match Peers. If specified, *all* labels must
	// be present on peers in order for them to match.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RouteBinding is the Schema for the routebindings API
// +k8s:openapi-gen=true
type RouteBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteBindingSpec   `json:"spec,omitempty"`
	Status RouteBindingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RouteBindingList contains a list of RouteBinding
type RouteBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RouteBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RouteBinding{}, &RouteBindingList{})
}

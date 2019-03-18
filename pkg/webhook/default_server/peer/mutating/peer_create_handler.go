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

package mutating

import (
	"context"
	"fmt"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"

	wgv1alpha1 "github.com/munnerz/kubewg/pkg/apis/wg/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

func init() {
	webhookName := "mutating-create-peer"
	if HandlerMap[webhookName] == nil {
		HandlerMap[webhookName] = []admission.Handler{}
	}
	HandlerMap[webhookName] = append(HandlerMap[webhookName], &PeerCreateHandler{})
}

// PeerCreateHandler handles Peer
type PeerCreateHandler struct {
	Client  client.Client

	// Decoder decodes objects
	Decoder types.Decoder
}

func (h *PeerCreateHandler) mutatingPeerFn(ctx context.Context, obj *wgv1alpha1.Peer) error {
	if obj.Spec.Network == "" {
		return fmt.Errorf("network must be specified")
	}
	if obj.Spec.Address != "" {
		return nil
	}

	// TODO(user): implement your admission logic
	return nil
}

var _ admission.Handler = &PeerCreateHandler{}

// Handle handles admission requests.
func (h *PeerCreateHandler) Handle(ctx context.Context, req types.Request) types.Response {
	obj := &wgv1alpha1.Peer{}

	err := h.Decoder.Decode(req, obj)
	if err != nil {
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}
	copy := obj.DeepCopy()

	err = h.mutatingPeerFn(ctx, copy)
	if err != nil {
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	return admission.PatchResponse(obj, copy)
}

var _ inject.Client = &PeerCreateHandler{}
//
//// InjectClient injects the client into the PeerCreateHandler
func (h *PeerCreateHandler) InjectClient(c client.Client) error {
	h.Client = c
	return nil
}

var _ inject.Decoder = &PeerCreateHandler{}

// InjectDecoder injects the decoder into the PeerCreateHandler
func (h *PeerCreateHandler) InjectDecoder(d types.Decoder) error {
	h.Decoder = d
	return nil
}

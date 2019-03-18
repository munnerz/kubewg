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

package network

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/munnerz/kubewg/pkg/api"
	wgv1alpha1 "github.com/munnerz/kubewg/pkg/apis/wg/v1alpha1"
)

var log = logf.Log.WithName("controller")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Network Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNetwork{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("network-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Network
	err = c.Watch(&source.Kind{Type: &wgv1alpha1.Network{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to Peers
	err = c.Watch(&source.Kind{Type: &wgv1alpha1.Peer{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: &peerMapper{
			Client: mgr.GetClient(),
		},
	})
	if err != nil {
		return err
	}

	return nil
}

type peerMapper struct {
	client.Client
}

func (m *peerMapper) Map(obj handler.MapObject) (reqs []reconcile.Request) {
	reqs = []reconcile.Request{}

	p := obj.Object.(*wgv1alpha1.Peer)
	if p.Status.Network == "" {
		return nil
	}

	var networks wgv1alpha1.NetworkList
	if err := m.Client.List(context.TODO(), &client.ListOptions{Namespace: obj.Meta.GetNamespace()}, &networks); err != nil {
		log.Error(err, "error listing networks when mapping peers")
		return
	}

	for _, n := range networks.Items {
		if p.Status.Network == n.Name {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: n.Namespace,
					Name:      n.Name,
				},
			})
		}
	}

	return reqs
}

var _ reconcile.Reconciler = &ReconcileNetwork{}

// ReconcileNetwork reconciles a Network object
type ReconcileNetwork struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Network object and makes changes based on the state read
// and what is in the Network.Spec
// +kubebuilder:rbac:groups=wg.mnrz.xyz,resources=networks;peers,verbs=get;list;watch
// +kubebuilder:rbac:groups=wg.mnrz.xyz,resources=networks/status,verbs=get;update;patch
func (r *ReconcileNetwork) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Network instance
	instance := &wgv1alpha1.Network{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	network := instance.DeepCopy()

	var peers wgv1alpha1.PeerList
	if err := r.Client.List(context.TODO(), &client.ListOptions{Namespace: network.Namespace}, &peers); err != nil {
		return reconcile.Result{}, err
	}

	networkPeers := map[string]struct{}{}
	allocatedIPs := map[string]string{}
	for _, alloc := range network.Status.Allocations {
		allocatedIPs[alloc.Name] = alloc.Address
	}

	// allocate IPs for any new peers
	for _, p := range peers.Items {
		// skip peers that aren't part of this network
		if p.Status.Network != network.Name {
			continue
		}
		networkPeers[p.Name] = struct{}{}

		allocIP, ok := allocatedIPs[p.Name]
		if !ok {
			allocIP, err = allocateIP(&p, network)
			if err != nil {
				log.Error(err, "error allocating IP for peer", "peer", p.Name)
				continue
			}
			log.Info("allocated IP address to peer", "peer", p.Name, "address", allocIP)
			network.Status.Allocations = append(network.Status.Allocations, wgv1alpha1.IPAssignment{
				Name:    p.Name,
				Address: allocIP,
			})
			continue
		}
	}

	var newAllocations []wgv1alpha1.IPAssignment
	// remove old IPs for peers that have been deleted
	for _, alloc := range network.Status.Allocations {
		if _, ok := networkPeers[alloc.Name]; ok {
			newAllocations = append(newAllocations, alloc)
		}
	}

	if !reflect.DeepEqual(instance.Status, network.Status) {
		log.Info("Updating Network", "namespace", network.Namespace, "name", network.Name)
		err = r.Status().Update(context.TODO(), network)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func allocateIP(p *wgv1alpha1.Peer, n *wgv1alpha1.Network) (string, error) {
	matchesRule := func(rule wgv1alpha1.AllocationRule) bool {
		if rule.Selector == nil {
			return true
		}

		return api.PeerMatchesSelector(p, *rule.Selector)
	}

	for _, r := range n.Spec.Allocations {
		if matchesRule(r) {
			if r.Address == nil {
				return "", nil
			}
			return *r.Address, nil
		}
	}

	return "", fmt.Errorf("failed to allocate address")
}

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

package peer

import (
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/diff"
	"reflect"
	"strings"
	"testing"
	"time"

	wgv1alpha1 "github.com/munnerz/kubewg/pkg/apis/wg/v1alpha1"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type testLogger struct {
	t      *testing.T
	names  []string
	values []interface{}
}

var _ logr.Logger = &testLogger{}

func (l *testLogger) log(msg string, lvl string, names []string, kv ...interface{}) {
	kv = append(l.values, kv...)
	names = append(l.names, names...)
	l.t.Logf("(%s) %s %s %v", strings.Join(names, "/"), lvl, msg, kv)
}

func (l *testLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	l.log(msg, "ERROR", nil, keysAndValues...)
}

func (l *testLogger) Enabled() bool {
	return true
}

func (l *testLogger) Info(msg string, keysAndValues ...interface{}) {
	l.log(msg, "INFO", nil, keysAndValues...)
}

func (l *testLogger) V(n int) logr.InfoLogger {
	return l
}

func (l *testLogger) WithName(s string) logr.Logger {
	lCpy := *l
	lCpy.names = append(lCpy.names, s)
	return &lCpy
}

func (l *testLogger) WithValues(keysAndValues ...interface{}) logr.Logger {
	lCpy := *l
	lCpy.values = append(lCpy.values, keysAndValues...)
	return &lCpy
}

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var expectedRequest2 = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo2", Namespace: "default"}}
var peerKey = types.NamespacedName{Name: "foo", Namespace: "default"}
var peerKey2 = types.NamespacedName{Name: "foo2", Namespace: "default"}

const timeout = time.Second * 5

func TestReconcileTwoPeers(t *testing.T) {
	log = &testLogger{t: t}
	g := gomega.NewGomegaWithT(t)
	instance := &wgv1alpha1.Peer{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec: wgv1alpha1.PeerSpec{
			PublicKey: "public",
			Endpoint:  ":12345",
		},
		Status: wgv1alpha1.PeerStatus{
			Address: "1.2.3.4/24",
			Network: "testnet",
		},
	}
	existingNetworks := []wgv1alpha1.Network{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "testnet", Namespace: "default"},
			Spec: wgv1alpha1.NetworkSpec{
				Subnet: "1.2.3.0/24",
			},
		},
	}
	existingPeers := []wgv1alpha1.Peer{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "foo2", Namespace: "default"},
			Spec: wgv1alpha1.PeerSpec{
				PublicKey: "publicpeer",
				Endpoint:  "example.com:12345",
			},
			Status: wgv1alpha1.PeerStatus{
				Address: "1.2.3.5/24",
				Network: "testnet",
			},
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Create the Peer object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	// The instance object may not be a valid object because it might be missing some required fields.
	// Please modify the instance object by adding required fields and then remove the following if statement.
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)

	for _, peer := range existingPeers {
		err = c.Create(context.TODO(), &peer)
		// The instance object may not be a valid object because it might be missing some required fields.
		// Please modify the instance object by adding required fields and then remove the following if statement.
		if apierrors.IsInvalid(err) {
			t.Logf("failed to create additional object, got an invalid object error: %v", err)
			return
		}
		g.Expect(err).NotTo(gomega.HaveOccurred())
		p := peer
		defer c.Delete(context.TODO(), &p)
	}
	for _, n := range existingNetworks {
		err = c.Create(context.TODO(), &n)
		// The instance object may not be a valid object because it might be missing some required fields.
		// Please modify the instance object by adding required fields and then remove the following if statement.
		if apierrors.IsInvalid(err) {
			t.Logf("failed to create additional object, got an invalid object error: %v", err)
			return
		}
		g.Expect(err).NotTo(gomega.HaveOccurred())
		p := n
		defer c.Delete(context.TODO(), &p)
	}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	expectedStatus1 := wgv1alpha1.PeerStatus{
		Address: "1.2.3.4/24",
		Network: "testnet",
		Peers: []wgv1alpha1.PeerConfiguration{
			{
				Name:       "foo2",
				PublicKey:  "publicpeer",
				Endpoint:   "example.com:12345",
				AllowedIPs: []string{"1.2.3.5/32"},
			},
		},
	}
	// Ensure the first peer has got a route config for the second peer
	g.Eventually(func() error {
		updatedPeer := &wgv1alpha1.Peer{}
		if err := c.Get(context.TODO(), peerKey, updatedPeer); err != nil {
			return err
		}
		if !reflect.DeepEqual(updatedPeer.Status, expectedStatus1) {
			return fmt.Errorf("unexpected difference: %s", diff.ObjectReflectDiff(updatedPeer.Status, expectedStatus1))
		}
		return nil
	}, timeout).Should(gomega.Succeed())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest2)))
	expectedStatus2 := wgv1alpha1.PeerStatus{
		Address: "1.2.3.5/24",
		Network: "testnet",
		Peers: []wgv1alpha1.PeerConfiguration{
			{
				Name:       "foo",
				PublicKey:  "public",
				AllowedIPs: []string{"1.2.3.4/32"},
			},
		},
	}
	// Ensure the first peer has got a route config for the second peer
	g.Eventually(func() error {
		updatedPeer := &wgv1alpha1.Peer{}
		if err := c.Get(context.TODO(), peerKey2, updatedPeer); err != nil {
			return err
		}
		if !reflect.DeepEqual(updatedPeer.Status, expectedStatus2) {
			return fmt.Errorf("unexpected difference: %s", diff.ObjectReflectDiff(updatedPeer.Status, expectedStatus2))
		}
		return nil
	}, timeout).Should(gomega.Succeed())
}

func TestReconcileTwoPeersOneRouteRule(t *testing.T) {
	log = &testLogger{t: t}
	g := gomega.NewGomegaWithT(t)
	instance := &wgv1alpha1.Peer{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec: wgv1alpha1.PeerSpec{
			PublicKey: "public",
			Endpoint:  ":12345",
		},
		Status: wgv1alpha1.PeerStatus{
			Address: "1.2.3.4/24",
			Network: "testnet",
		},
	}
	existingNetworks := []wgv1alpha1.Network{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "testnet", Namespace: "default"},
			Spec: wgv1alpha1.NetworkSpec{
				Subnet: "1.2.3.0/24",
			},
		},
	}
	existingPeers := []wgv1alpha1.Peer{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "foo2", Namespace: "default"},
			Spec: wgv1alpha1.PeerSpec{
				PublicKey: "publicpeer",
				Endpoint:  "example.com:12345",
			},
			Status: wgv1alpha1.PeerStatus{
				Address: "1.2.3.5/24",
				Network: "testnet",
			},
		},
	}
	// send all traffic for 9.9.9.9/32 via foo2
	existingRouteRules := []wgv1alpha1.RouteBinding{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "foo2-rr", Namespace: "default"},
			Spec: wgv1alpha1.RouteBindingSpec{
				Routes: []string{"9.9.9.9/32"},
				Selector: wgv1alpha1.PeerSelector{
					Names: []string{"foo2"},
				},
			},
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Create the Peer object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	// The instance object may not be a valid object because it might be missing some required fields.
	// Please modify the instance object by adding required fields and then remove the following if statement.
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)

	for _, peer := range existingPeers {
		err = c.Create(context.TODO(), &peer)
		// The instance object may not be a valid object because it might be missing some required fields.
		// Please modify the instance object by adding required fields and then remove the following if statement.
		if apierrors.IsInvalid(err) {
			t.Logf("failed to create additional object, got an invalid object error: %v", err)
			return
		}
		g.Expect(err).NotTo(gomega.HaveOccurred())
		p := peer
		defer c.Delete(context.TODO(), &p)
	}
	for _, n := range existingNetworks {
		err = c.Create(context.TODO(), &n)
		// The instance object may not be a valid object because it might be missing some required fields.
		// Please modify the instance object by adding required fields and then remove the following if statement.
		if apierrors.IsInvalid(err) {
			t.Logf("failed to create additional object, got an invalid object error: %v", err)
			return
		}
		g.Expect(err).NotTo(gomega.HaveOccurred())
		p := n
		defer c.Delete(context.TODO(), &p)
	}
	for _, n := range existingRouteRules {
		err = c.Create(context.TODO(), &n)
		// The instance object may not be a valid object because it might be missing some required fields.
		// Please modify the instance object by adding required fields and then remove the following if statement.
		if apierrors.IsInvalid(err) {
			t.Logf("failed to create additional object, got an invalid object error: %v", err)
			return
		}
		g.Expect(err).NotTo(gomega.HaveOccurred())
		p := n
		defer c.Delete(context.TODO(), &p)
	}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	expectedStatus1 := wgv1alpha1.PeerStatus{
		Address: "1.2.3.4/24",
		Network: "testnet",
		Peers: []wgv1alpha1.PeerConfiguration{
			{
				Name:       "foo2",
				PublicKey:  "publicpeer",
				Endpoint:   "example.com:12345",
				AllowedIPs: []string{"1.2.3.5/32", "9.9.9.9/32"},
			},
		},
	}
	// Ensure the first peer has got a route config for the second peer
	g.Eventually(func() error {
		updatedPeer := &wgv1alpha1.Peer{}
		if err := c.Get(context.TODO(), peerKey, updatedPeer); err != nil {
			return err
		}
		if !reflect.DeepEqual(updatedPeer.Status, expectedStatus1) {
			return fmt.Errorf("unexpected difference: %s", diff.ObjectReflectDiff(updatedPeer.Status, expectedStatus1))
		}
		return nil
	}, timeout).Should(gomega.Succeed())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest2)))
	expectedStatus2 := wgv1alpha1.PeerStatus{
		Address: "1.2.3.5/24",
		Network: "testnet",
		Peers: []wgv1alpha1.PeerConfiguration{
			{
				Name:       "foo",
				PublicKey:  "public",
				AllowedIPs: []string{"1.2.3.4/32"},
			},
		},
	}
	// Ensure the first peer has got a route config for the second peer
	g.Eventually(func() error {
		updatedPeer := &wgv1alpha1.Peer{}
		if err := c.Get(context.TODO(), peerKey2, updatedPeer); err != nil {
			return err
		}
		if !reflect.DeepEqual(updatedPeer.Status, expectedStatus2) {
			return fmt.Errorf("unexpected difference: %s", diff.ObjectReflectDiff(updatedPeer.Status, expectedStatus2))
		}
		return nil
	}, timeout).Should(gomega.Succeed())
}

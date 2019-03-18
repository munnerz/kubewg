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

package guardlet

import (
	"context"
	"fmt"
	"github.com/munnerz/kubewg/pkg/config"
	"net"
	"os/exec"
	"strings"

	wgv1alpha1 "github.com/munnerz/kubewg/pkg/apis/wg/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Peer Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePeer{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("peer-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Peer
	err = c.Watch(&source.Kind{Type: &wgv1alpha1.Peer{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePeer{}

// ReconcilePeer reconciles a Peer object
type ReconcilePeer struct {
	client.Client
	scheme *runtime.Scheme
}

func buildCfgArgs(iface string, p *wgv1alpha1.Peer) ([]string, error) {
	args := []string{"set", iface}
	_, port, err := net.SplitHostPort(p.Spec.Endpoint)
	if err != nil {
		return nil, err
	}
	args = append(args, "listen-port", port, "private-key", config.PrivateKeyFile)
	for _, c := range p.Status.Peers {
		if c.PublicKey == "" {
			return nil, fmt.Errorf("no peer public key provided for %q", c.Name)
		}
		peerCfg := []string{"peer", c.PublicKey}
		// TODO: handle preshared keys
		if c.Endpoint != "" {
			host, port, err := net.SplitHostPort(c.Endpoint)
			if err != nil {
				return nil, fmt.Errorf("invalid peer endpoint: %v", err)
			}
			peerCfg = append(peerCfg, "endpoint", fmt.Sprintf("%s:%s", host, port))
		}
		if len(c.AllowedIPs) == 0 {
			args = append(args, peerCfg...)
			continue
		}
		peerCfg = append(peerCfg, "allowed-ips", strings.Join(c.AllowedIPs, ","))
		args = append(args, peerCfg...)
		continue
	}
	return args, nil
}

func setupInterface() error {
	existingIface, _ := net.InterfaceByName(config.DeviceName)
	if existingIface != nil {
		log.Info("skipping interface setup as existing interface already exists", "interface", config.DeviceName)
		return nil
	}
	if !config.UseKernelModule {
		// use wireguard-go to setup the tunnel
		output, err := exec.Command("wireguard-go", config.DeviceName).CombinedOutput()
		if err != nil {
			log.Error(err, "error running wireguard-go", "output", string(output))
			return nil
		}
		return nil
	}

	output, err := exec.Command("ip", "link", "add", config.DeviceName, "type", "wireguard").CombinedOutput()
	if err != nil {
		log.Error(err, "error adding network interface", "output", string(output))
		return nil
	}

	return nil
}

func setupIP(ipMask string) error {
	var ip string
	idx := strings.Index(ipMask, "/")
	if idx == -1 {
		ip = ipMask
		ipMask = ipMask + "/32"
	} else {
		ip = ipMask[:idx]
	}
	existingIface, err := net.InterfaceByName(config.DeviceName)
	if err != nil {
		return fmt.Errorf("failed to lookup interface %q: %v", config.DeviceName, err)
	}
	addrs, err := existingIface.Addrs()
	if err != nil {
		return fmt.Errorf("failed to lookup IPs for interface %q: %v", config.DeviceName, err)
	}
	for _, addr := range addrs {
		log.Info("existing interface IP found", "ip", addr.String())
		if addr.String() == ipMask {
			log.Info("existing address already exists, skipping configuring IP", "interface", config.DeviceName, "ip", ipMask)
			return nil
		}
	}
	log.Info("configuring IP", "subnet", ipMask, "ip", ip)
	var cmd *exec.Cmd
	switch config.OS {
	case "darwin":
		cmd = exec.Command("ifconfig", config.DeviceName, "inet", ipMask, ip)
	case "linux":
		cmd = exec.Command("ip", "addr", "add", ipMask, "dev", config.DeviceName)
	default:
		return fmt.Errorf("unsupported os %q", config.OS)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error configuring IP (%q) (%v): %s", ipMask, err, string(output))
	}
	return nil
}

func addRoutes(ipMask string, peers []wgv1alpha1.PeerConfiguration) error {
	var ip string
	idx := strings.Index(ipMask, "/")
	if idx == -1 {
		ip = ipMask
		ipMask = ipMask + "/32"
	} else {
		ip = ipMask[:idx]
	}
	for _, p := range peers {
		for _, subnet := range p.AllowedIPs {
			log.Info("configuring route", "subnet", subnet)
			var cmd *exec.Cmd
			switch config.OS {
			case "darwin":
				cmd = exec.Command("route", "-q", "-n", "add", "-inet", subnet, "-interface", config.DeviceName)
			case "linux":
				cmd = exec.Command("ip", "route", "add", subnet, "via", ip, "dev", config.DeviceName)
			default:
				return fmt.Errorf("unsupported os %q", config.OS)
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Error(err, "error configuring route", "peer", p.Name, "route", subnet, "output", string(output))
				continue
			}
		}
	}
	return nil
}

func ifup(dev string) error {
	var cmd *exec.Cmd
	switch config.OS {
	case "darwin":
		cmd = exec.Command("ifconfig", dev, "up")
	case "linux":
		cmd = exec.Command("ip", "link", "set", dev, "up")
	default:
		return fmt.Errorf("unsupported os %q", config.OS)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error bringing up interface %q (%v): %s", dev, err, string(output))
	}
	return nil
}

// Reconcile reads that state of the cluster for a Peer object and makes changes based on the state read
// and what is in the Peer.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=wg.mnrz.xyz,resources=peers,verbs=get;list;watch
// +kubebuilder:rbac:groups=wg.mnrz.xyz,resources=peers/status,verbs=get;update;patch
func (r *ReconcilePeer) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	if request.Name != config.PeerName {
		return reconcile.Result{}, nil
	}
	log.Info("Reconciling wireguard configuration")
	// Fetch the Peer instance
	instance := &wgv1alpha1.Peer{}
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

	log.Info("Setting up wireguard network interface")
	if err := setupInterface(); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Building wireguard configuration arguments")
	// TODO: call add_iface to determine iface name
	args, err := buildCfgArgs(config.DeviceName, instance)
	if err != nil {
		// TODO: absorb and log error
		return reconcile.Result{}, err
	}

	cmd := exec.Command(config.WGBinary, args...)
	log.Info("Executing command", "cmd", config.WGBinary, "args", args)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Error(err, "error running command", "output", string(output))
		return reconcile.Result{}, err
	}

	log.Info("Configuring IP")
	if err := setupIP(instance.Status.Address); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Bringing up interface", "interface", config.DeviceName)
	if err := ifup(config.DeviceName); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Configuring routes")
	if err := addRoutes(instance.Status.Address, instance.Status.Peers); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Reconciled wireguard configuration")

	return reconcile.Result{}, nil
}

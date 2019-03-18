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

package main

import (
	"context"
	"flag"
	"net"
	"os"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/munnerz/kubewg/pkg/apis"
	wgv1alpha1 "github.com/munnerz/kubewg/pkg/apis/wg/v1alpha1"
)

var scheme = runtime.NewScheme()

var (
	privateKey    = flag.String("private-key", "", "private key to use in the generated config")
	peerName      = flag.String("peer-name", "", "name of the peer to get config for")
	peerNamespace = flag.String("peer-namespace", "", "namespace of the peer to get config for")
)

func main() {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))
	log := logf.Log.WithName("mkconf")

	log.Info("using private key", "key", *privateKey)
	// Get a config to talk to the apiserver
	//log.Info("setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Setup Scheme for all resources
	//log.Info("setting up scheme")
	if err := apis.AddToScheme(scheme); err != nil {
		log.Error(err, "unable add APIs to scheme")
		os.Exit(1)
	}

	cl, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Error(err, "error creating client")
		os.Exit(1)
	}

	var p wgv1alpha1.Peer
	if err := cl.Get(context.TODO(), client.ObjectKey{
		Namespace: *peerNamespace,
		Name:      *peerName,
	}, &p); err != nil {
		log.Error(err, "error getting peer")
		os.Exit(1)
	}

	err = generateConf(&p)
	if err != nil {
		log.Error(err, "error generating config")
		os.Exit(1)
	}
}

func generateConf(p *wgv1alpha1.Peer) error {
	_, port, err := net.SplitHostPort(p.Spec.Endpoint)
	if err != nil {
		return err
	}
	var peers []peerData
	for _, peer := range p.Status.Peers {
		peers = append(peers, peerData{
			PublicKey:  peer.PublicKey,
			AllowedIPs: strings.Join(peer.AllowedIPs, ","),
			Endpoint:   peer.Endpoint,
		})
	}
	data := gotmpldata{
		Address:    p.Status.Address + "/32",
		ListenPort: port,
		PrivateKey: *privateKey,
		Peers:      peers,
	}
	t, err := template.New("peerconfig").Parse(gotmpl)
	if err != nil {
		return err
	}

	err = t.Execute(os.Stdout, data)
	if err != nil {
		return err
	}

	return nil
}

type gotmpldata struct {
	Address    string
	ListenPort string
	PrivateKey string
	Peers      []peerData
}

type peerData struct {
	PublicKey  string
	AllowedIPs string
	Endpoint   string
}

var gotmpl = `[Interface]
Address = {{.Address}}
ListenPort = {{.ListenPort}}
PrivateKey = {{ printf "%s" .PrivateKey}}

{{ range .Peers }}
[Peer]
PublicKey = {{.PublicKey}}
AllowedIPs = {{.AllowedIPs}}
{{ if .Endpoint -}}
Endpoint = {{ .Endpoint }}
{{ end -}}
PersistentKeepalive = 15
{{ end }}
`

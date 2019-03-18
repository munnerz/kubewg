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
	"flag"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/munnerz/kubewg/pkg/apis"
	cfg "github.com/munnerz/kubewg/pkg/config"
	"github.com/munnerz/kubewg/pkg/controller/guardlet"
)

func main() {
	var metricsAddr string
	var syncPeriod time.Duration
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.DurationVar(&syncPeriod, "sync-period", time.Second*5, "Adjust how often interface configuration is periodically resynced")
	cfg.AddFlags(flag.CommandLine)
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))
	log := logf.Log.WithName("entrypoint")

	if err := cfg.Complete(); err != nil {
		log.Error(err, "invalid flags")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	log.Info("setting up client for guardlet")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	log.Info("setting up guardlet")
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: metricsAddr,
		SyncPeriod:         &syncPeriod,
	})
	if err != nil {
		log.Error(err, "unable to set up guardlet")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	log.Info("setting up scheme")
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add APIs to scheme")
		os.Exit(1)
	}

	// Setup all Controllers
	log.Info("Setting up controller")
	if err := guardlet.Add(mgr); err != nil {
		log.Error(err, "unable to register controllers to the guardlet")
		os.Exit(1)
	}

	// Start the Cmd
	log.Info("Starting the Cmd.")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "unable to run the manager")
		os.Exit(1)
	}
}

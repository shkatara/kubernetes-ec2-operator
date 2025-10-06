/*
Copyright 2025.

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

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	computev1 "github.com/shkatara/ec2Operator/api/v1"
	"github.com/shkatara/ec2Operator/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// The init function is used to initialize the scheme variable with the Kubernetes client-go scheme
// and the custom API scheme (computev1). This ensures that the manager knows about all the resource
// types it needs to handle. The utilruntime.Must function is used to panic if adding a scheme fails,
// which is a common pattern in controller-runtime projects to catch programming errors early.
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme)) // Registers built-in Kubernetes types

	utilruntime.Must(computev1.AddToScheme(scheme)) // Registers custom resource types for this operator
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var probeAddr string
	var metricsAddr string
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metrics endpoint binds to.")

	opts := zap.Options{
		Development: true,
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Create watcher for webhook certificates
	// webhookCertWatcher is a pointer to a CertWatcher, which can be used to watch for changes
	// in webhook TLS certificates and reload them automatically. This is useful for supporting
	// certificate rotation without restarting the manager. If not used, it remains nil.
	var webhookCertWatcher *certwatcher.CertWatcher

	// Create a new webhook server. The webhook server is responsible for serving admission webhooks
	// (such as mutating or validating webhooks) for custom resources. The TLSOpts field can be used
	// to customize TLS configuration, but is set to nil here for default behavior.
	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: nil,
	})

	// Create a new controller-runtime Manager. The Manager is the main entry point for running controllers,
	// webhooks, and other background tasks. It is configured with the scheme (which defines the types it knows about),
	// the webhook server, and the address for health probes. ctrl.GetConfigOrDie() loads the Kubernetes REST config.
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Set up the Ec2InstanceReconciler controller with the manager.
	// This controller will watch and reconcile Ec2Instance custom resources.
	if err = (&controller.Ec2InstanceReconciler{
		Client: mgr.GetClient(), // Kubernetes client for interacting with API server
		Scheme: mgr.GetScheme(), // Scheme defines the types the client can work with
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Ec2Instance")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	// If a webhook certificate watcher is configured, add it to the manager.
	// This ensures the manager can reload webhook certificates automatically.
	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	// Add a health check endpoint to the manager.
	// This endpoint can be used by Kubernetes to check if the manager is healthy.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	// Add a readiness check endpoint to the manager.
	// This endpoint can be used by Kubernetes to check if the manager is ready to serve requests.
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Start the manager, which will block until the process receives a termination signal.
	// The manager handles all registered controllers, webhooks, and background tasks.
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

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

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	computev1 "github.com/shkatara/ec2Operator/api/v1"
)

// Ec2InstanceReconciler is a struct that implements the logic for reconciling Ec2Instance custom resources.
// It embeds the Kubernetes client.Client interface, which provides methods for interacting with the Kubernetes API server,
// and holds a pointer to a runtime.Scheme, which is used for type conversions between Go structs and Kubernetes objects.
// This struct is used to reconcile the Ec2Instance custom resource.

type Ec2InstanceReconciler struct {
	client.Client                 // Used to perform CRUD operations on Kubernetes resources.
	Scheme        *runtime.Scheme // Used to map Go types to Kubernetes GroupVersionKinds and vice versa.
}

// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Ec2Instance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *Ec2InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	fmt.Println("A new ec2 instance is created")

	// handle when resource is deleted
	if req.Name != "" {
		// Create a new instance of the Ec2Instance struct to hold the data retrieved from the Kubernetes API.
		// This struct will be populated with the current state of the Ec2Instance resource specified by the request.
		ec2Instance := &computev1.Ec2Instance{}
		// Retrieve the Ec2Instance resource from the Kubernetes API server using the provided request's NamespacedName.
		err := r.Get(ctx, req.NamespacedName, ec2Instance)
		fmt.Println(ec2Instance)

		if err != nil {
			return ctrl.Result{}, err
		}

		//print the name
		fmt.Println(ec2Instance.Name, ec2Instance.Namespace, ec2Instance.Spec.InstanceType)
	}

	// The Reconcile function must return a ctrl.Result and an error.
	// Returning ctrl.Result{} with nil error means the reconciliation was successful
	// and no requeue is requested. If an error is returned, the controller will
	// automatically requeue the request for another attempt.
	// Sends Requeue ( bool ) and RequeueAfter ( time.Duration ).
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Ec2InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1.Ec2Instance{}).
		Named("ec2instance").
		Complete(r)
}

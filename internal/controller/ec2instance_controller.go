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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	computev1 "github.com/shkatara/ec2Operator/api/v1"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
//
// After updating the status of the resource (e.g., with r.Status().Update), the Kubernetes API server
// will emit an update event for the resource. This event will be picked up by the controller-runtime
// and will cause the Reconcile function to be called again for the same resource. This is why, after
// updating the status, the reconciler is called again: it is a result of the Kubernetes watch mechanism
// and ensures that the controller can observe and react to any changes, including those it made itself.
// This pattern is common in Kubernetes controllers to ensure eventual consistency and to handle
// situations where the status update may not have been fully applied or observed yet.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *Ec2InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// TODO(user): your logic here
	l.Info("Ec2Instance Reconciler recieved a request")

	// Create a new instance of the Ec2Instance struct to hold the data retrieved from the Kubernetes API.
	// This struct will be populated with the current state of the Ec2Instance resource specified by the request.
	// Retrieve the Ec2Instance resource from the Kubernetes API server using the provided request's NamespacedName.
	ec2Instance := &computev1.Ec2Instance{}
	if err := r.Get(ctx, req.NamespacedName, ec2Instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	fmt.Println("Instance ID is now", ec2Instance.Status.InstanceID)

	// Check if we already have an instance ID in status
	if ec2Instance.Status.InstanceID != "" {
		// Instance already exists, verify it's still running
		instanceExist, instanceState, _ := checkEC2InstanceExists(ctx, ec2Instance.Status.InstanceID, ec2Instance)
		// if err != nil {
		// 	// Instance might be terminated, clear status and recreate
		// 	ec2Instance.Status.InstanceID = ""
		// 	ec2Instance.Status.State = ""
		// 	ec2Instance.Status.PublicIP = ""
		// 	ec2Instance.Status.PrivateIP = ""
		// 	ec2Instance.Status.PublicDNS = ""
		// 	ec2Instance.Status.PrivateDNS = ""
		// 	return ctrl.Result{Requeue: true}, r.Status().Update(ctx, ec2Instance)
		// }
		if instanceExist {
			// check status of the instance is running
			if instanceState.State.Name != ec2types.InstanceStateNameRunning {
				// update the status of the instance
				ec2Instance.Status.State = string(instanceState.State.Name)
				return ctrl.Result{}, r.Status().Update(ctx, ec2Instance)
			}
			// Instance exists, we're done
			return ctrl.Result{}, nil
		}
		// Instance does not exist, we're done
		return ctrl.Result{}, nil
	}

	createdInstanceInfo, err := createEc2Instance(ec2Instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	ec2Instance.Status.InstanceID = createdInstanceInfo.InstanceID
	ec2Instance.Status.State = createdInstanceInfo.State
	ec2Instance.Status.PublicIP = createdInstanceInfo.PublicIP
	ec2Instance.Status.PrivateIP = createdInstanceInfo.PrivateIP
	ec2Instance.Status.PublicDNS = createdInstanceInfo.PublicDNS
	ec2Instance.Status.PrivateDNS = createdInstanceInfo.PrivateDNS

	// The Reconcile function must return a ctrl.Result and an error.
	// Returning ctrl.Result{} with nil error means the reconciliation was successful
	// and no requeue is requested. If an error is returned, the controller will
	// automatically requeue the request for another attempt.
	// Sends Requeue ( bool ) and RequeueAfter ( time.Duration ).
	//return ctrl.Result{}, nil
	return ctrl.Result{}, r.Status().Update(ctx, ec2Instance)

}

// SetupWithManager sets up the controller with the Manager.
func (r *Ec2InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1.Ec2Instance{}).
		Named("ec2instance").
		Complete(r)
}

func createEc2Instance(ec2Instance *computev1.Ec2Instance) (createdInstanceInfo *computev1.CreatedInstanceInfo, err error) {

	// create the client for ec2 instance
	ec2Client := awsClient(ec2Instance.Spec.Region)

	// create the input for the run instances
	runInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(ec2Instance.Spec.AMIId),
		InstanceType: ec2types.InstanceType(ec2Instance.Spec.InstanceType),
		KeyName:      aws.String(ec2Instance.Spec.KeyPair),
		SubnetId:     aws.String(ec2Instance.Spec.Subnet),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		//SecurityGroupIds: []string{ec2Instance.Spec.SecurityGroups[0]},
	}

	// run the instances
	result, err := ec2Client.RunInstances(context.TODO(), runInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create EC2 instance: %w", err)
	}

	if len(result.Instances) == 0 {
		fmt.Println("No instances returned in RunInstancesOutput")
		return nil, nil
	}
	// Till here, the instance is created and we have
	// Instance ID, private dns and IP, instance type and image id.
	inst := result.Instances[0]

	// Now we need to wait for the instance to be running and then get the public ip and dns
	time.Sleep(10 * time.Second)

	// After creating the instance, we waited and now we describe to
	// 1. Get the public IP and dns as it takes some time for it
	// 2. Getting the state of the instance.
	// We do this so we can send the instance's state to the status of the custom resource. for user to see with kubectl get ec2instances
	describeInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{*inst.InstanceId},
	}

	describeResult, err := ec2Client.DescribeInstances(context.TODO(), describeInput)
	if err != nil {
		return nil, fmt.Errorf("failed to describe EC2 instance: %w", err)
	}

	fmt.Println("Describe result", "public ip", *describeResult.Reservations[0].Instances[0].PublicDnsName, "state", describeResult.Reservations[0].Instances[0].State.Name)

	// You get "invalid memory address or nil pointer dereference" here if any of the following are true:
	// - result.Instances is nil or has length 0
	// - Any of the pointer fields (e.g., PublicIpAddress, PrivateIpAddress, etc.) are nil

	// To avoid this, always check for nil and length before dereferencing:

	// Wait for a bit to allow instance fields to be populated

	fmt.Printf("Private IP of the instance: %v\n", derefString(inst.PrivateIpAddress))
	fmt.Printf("State of the instance: %v\n", describeResult.Reservations[0].Instances[0].State.Name)
	fmt.Printf("Private DNS of the instance: %v\n", derefString(inst.PrivateDnsName))
	fmt.Printf("Instance ID of the instance: %v\n", derefString(inst.InstanceId))
	fmt.Println("Instance Type of the instance: ", inst.InstanceType)
	fmt.Printf("Image ID of the instance: %v\n", derefString(inst.ImageId))
	fmt.Printf("Key Name of the instance: %v\n", derefString(inst.KeyName))

	// block until the instance is running
	// blockUntilInstanceRunning(ctx, ec2Instance.Status.InstanceID, ec2Instance)
	createdInstanceInfo = &computev1.CreatedInstanceInfo{
		InstanceID: *inst.InstanceId,
		State:      string(describeResult.Reservations[0].Instances[0].State.Name),
		PublicIP:   *describeResult.Reservations[0].Instances[0].PublicIpAddress,
		PrivateIP:  *describeResult.Reservations[0].Instances[0].PrivateIpAddress,
		PublicDNS:  *describeResult.Reservations[0].Instances[0].PublicDnsName,
		PrivateDNS: *describeResult.Reservations[0].Instances[0].PrivateDnsName,
	}

	// Optionally, update ec2Instance.Status.InstanceID = *result.Instances[0].InstanceId

	// For now, just return nil to indicate success.
	return createdInstanceInfo, nil
}

// derefString is a helper function to safely dereference *string
func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return "<nil>"
}

func checkEC2InstanceExists(ctx context.Context, instanceID string, ec2Instance *computev1.Ec2Instance) (bool, *ec2types.Instance, error) {
	if instanceID == "" {
		return false, nil, nil
	}
	// create the client for ec2 instance
	ec2Client := awsClient(ec2Instance.Spec.Region)

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "InvalidInstanceID.NotFound") {
			return false, nil, nil
		}
		return false, nil, err
	}

	// Check if we got any instances back
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			// Check if instance is not terminated
			if instance.State.Name != ec2types.InstanceStateNameTerminated &&
				instance.State.Name != ec2types.InstanceStateNameShuttingDown {
				return true, &instance, nil
			}
		}
	}

	return false, nil, nil
}

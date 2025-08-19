package controller

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	computev1 "github.com/shkatara/ec2Operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func deleteEc2Instance(ctx context.Context, ec2Instance *computev1.Ec2Instance) (bool, error) {
	l := log.FromContext(ctx)

	l.Info("Deleting EC2 instance", "instanceID", ec2Instance.Status.InstanceID)

	// create the client for ec2 instance
	ec2Client := awsClient(ec2Instance.Spec.Region)

	// Terminate the instance
	terminateResult, err := ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{ec2Instance.Status.InstanceID},
	})

	if err != nil {
		l.Error(err, "Failed to terminate EC2 instance")
		return false, err
	}

	l.Info("Instance termination initiated",
		"instanceID", ec2Instance.Status.InstanceID,
		"currentState", terminateResult.TerminatingInstances[0].CurrentState.Name)

	// Use the AWS SDK v2 waiter to efficiently wait for instance termination
	// The waiter uses exponential backoff and is more efficient than manual polling
	waiter := ec2.NewInstanceTerminatedWaiter(ec2Client)

	// Configure waiter options
	maxWaitTime := 5 * time.Minute // Maximum time to wait for termination
	waitParams := &ec2.DescribeInstancesInput{
		InstanceIds: []string{ec2Instance.Status.InstanceID},
	}

	l.Info("Waiting for instance to be terminated",
		"instanceID", ec2Instance.Status.InstanceID,
		"maxWaitTime", maxWaitTime)

	// Wait for the instance to be terminated
	err = waiter.Wait(ctx, waitParams, maxWaitTime)

	if err != nil {
		l.Error(err, "Failed while waiting for instance termination",
			"instanceID", ec2Instance.Status.InstanceID)
		return false, err
	}

	l.Info("EC2 instance successfully terminated", "instanceID", ec2Instance.Status.InstanceID)
	return true, nil
}

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	computev1 "github.com/shkatara/ec2Operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func deleteEc2Instance(ctx context.Context, ec2Instance *computev1.Ec2Instance) (bool, error) {
	l := log.FromContext(ctx)

	l.Info("Deleting EC2 instance", "instanceID", ec2Instance.Status.InstanceID)

	// create the client for ec2 instance
	ec2Client := awsClient(ec2Instance.Spec.Region)

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{ec2Instance.Status.InstanceID},
	}

	// delete the instance
	terminateResult, err := ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{ec2Instance.Status.InstanceID},
	})

	if err != nil {
		l.Error(err, "Failed to delete EC2 instance")
		return false, err
	}
	// this gives "shutting-down"
	fmt.Println("Instance state before sleep", terminateResult.TerminatingInstances[0].CurrentState.Name)

	// check if the instance is deleted. probe again after 10 seconds
	// if terminateResult.TerminatingInstances[0].CurrentState.Name == ec2types.InstanceStateNameTerminated {
	// 	l.Info("EC2 instance not found", "instanceID", ec2Instance.Status.InstanceID)
	// 	return false, nil
	// }

	// wait for 20 seconds for instance to be terminated
	// time.Sleep(20 * time.Second)

	// result, err := ec2Client.DescribeInstances(ctx, input)
	// if err != nil {
	// 	// Check if it's a "not found" error
	// 	fmt.Println("Error: describe instances", err.Error())
	// 	if strings.Contains(err.Error(), "InvalidInstanceID.NotFound") {
	// 		return false, nil
	// 	}
	// 	return false, err
	// }

	// fmt.Println("Instance post describe is", result.Reservations[0].Instances[0].State.Name)

	// for loop to check if the instance is deleted
	for i := 0; i < 10; i++ {
		fmt.Println("Probe number ", i, "for instance", ec2Instance.Status.InstanceID)
		result, _ := ec2Client.DescribeInstances(ctx, input)
		fmt.Println("Instance state on is", result.Reservations[0].Instances[0].State.Name)
		if result.Reservations[0].Instances[0].State.Name == ec2types.InstanceStateNameTerminated {
			l.Info("EC2 instance deleted from AWS", "instanceID", ec2Instance.Status.InstanceID)
			return true, nil
		}
		fmt.Println("Instance is not deleted yet. Sleeping for 10 seconds")
		time.Sleep(10 * time.Second)
		fmt.Println("Probing Again")
	}

	return false, nil
}

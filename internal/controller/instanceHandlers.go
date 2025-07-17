package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	computev1 "github.com/shkatara/ec2Operator/api/v1"
)

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

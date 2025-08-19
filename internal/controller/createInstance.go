package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	computev1 "github.com/shkatara/ec2Operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func createEc2Instance(ec2Instance *computev1.Ec2Instance) (createdInstanceInfo *computev1.CreatedInstanceInfo, err error) {
	l := log.Log.WithName("createEc2Instance")

	l.Info("=== STARTING EC2 INSTANCE CREATION ===",
		"ami", ec2Instance.Spec.AMIId,
		"instanceType", ec2Instance.Spec.InstanceType,
		"region", ec2Instance.Spec.Region)

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

	l.Info("=== CALLING AWS RunInstances API ===")
	// run the instances
	result, err := ec2Client.RunInstances(context.TODO(), runInput)
	if err != nil {
		l.Error(err, "Failed to create EC2 instance")
		return nil, fmt.Errorf("failed to create EC2 instance: %w", err)
	}

	if len(result.Instances) == 0 {
		l.Error(nil, "No instances returned in RunInstancesOutput")
		fmt.Println("No instances returned in RunInstancesOutput")
		return nil, nil
	}

	// Till here, the instance is created and we have
	// Instance ID, private dns and IP, instance type and image id.
	inst := result.Instances[0]
	l.Info("=== EC2 INSTANCE CREATED SUCCESSFULLY ===", "instanceID", *inst.InstanceId)

	// Now we need to wait for the instance to be running and then get the public ip and dns
	l.Info("=== WAITING FOR INSTANCE TO BE RUNNING ===")

	runWaiter := ec2.NewInstanceRunningWaiter(ec2Client)
	maxWaitTime := 3 * time.Minute // Increased from 10 seconds - instances typically take 30-60 seconds

	err = runWaiter.Wait(context.TODO(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{*inst.InstanceId},
	}, maxWaitTime)
	if err != nil {
		l.Error(err, "Failed to wait for instance to be running")
		return nil, fmt.Errorf("failed to wait for instance to be running: %w", err)
	}

	// After creating the instance, we waited and now we describe to
	// 1. Get the public IP and dns as it takes some time for it
	// 2. Getting the state of the instance.
	// We do this so we can send the instance's state to the status of the custom resource. for user to see with kubectl get ec2instances
	l.Info("=== CALLING AWS DescribeInstances API TO GET INSTANCE DETAILS ===")
	describeInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{*inst.InstanceId},
	}

	describeResult, err := ec2Client.DescribeInstances(context.TODO(), describeInput)
	if err != nil {
		l.Error(err, "Failed to describe EC2 instance")
		return nil, fmt.Errorf("failed to describe EC2 instance: %w", err)
	}

	fmt.Println("Describe result", "public ip", *describeResult.Reservations[0].Instances[0].PublicDnsName, "state", describeResult.Reservations[0].Instances[0].State.Name)

	// You get "invalid memory address or nil pointer dereference" here if any of the following are true:
	// - result.Instances is nil or has length 0
	// - Any of the pointer fields (e.g., PublicIpAddress, PrivateIpAddress, etc.) are nil

	// To avoid this, always check for nil and length before dereferencing:

	// Wait for a bit to allow instance fields to be populated

	fmt.Printf("Private IP of the instance: %v", derefString(inst.PrivateIpAddress))
	fmt.Printf("State of the instance: %v", describeResult.Reservations[0].Instances[0].State.Name)
	fmt.Printf("Private DNS of the instance: %v", derefString(inst.PrivateDnsName))
	fmt.Printf("Instance ID of the instance: %v", derefString(inst.InstanceId))
	fmt.Println("Instance Type of the instance: ", inst.InstanceType)
	fmt.Printf("Image ID of the instance: %v", derefString(inst.ImageId))
	fmt.Printf("Key Name of the instance: %v", derefString(inst.KeyName))

	// block until the instance is running
	// blockUntilInstanceRunning(ctx, ec2Instance.Status.InstanceID, ec2Instance)

	// Get the instance details safely (public IP/DNS might be nil for private subnets)
	instance := describeResult.Reservations[0].Instances[0]
	createdInstanceInfo = &computev1.CreatedInstanceInfo{
		InstanceID: *inst.InstanceId,
		State:      string(instance.State.Name),
		PublicIP:   derefString(instance.PublicIpAddress),
		PrivateIP:  derefString(instance.PrivateIpAddress),
		PublicDNS:  derefString(instance.PublicDnsName),
		PrivateDNS: derefString(instance.PrivateDnsName),
	}

	l.Info("=== EC2 INSTANCE CREATION COMPLETED ===",
		"instanceID", createdInstanceInfo.InstanceID,
		"state", createdInstanceInfo.State,
		"publicIP", createdInstanceInfo.PublicIP)

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

package controller

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	computev1 "github.com/shkatara/ec2Operator/api/v1"
)

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

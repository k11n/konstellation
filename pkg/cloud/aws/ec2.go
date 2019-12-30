package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
)

func ListSecurityGroups(ec2Svc *ec2.EC2, vpcId string) ([]*ec2.SecurityGroup, error) {
	sgResp, err := ec2Svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{VPCFilter(vpcId)},
	})
	if err != nil {
		return nil, err
	}

	return sgResp.SecurityGroups, nil
}

func ListSubnets(ec2Svc *ec2.EC2, vpcId string) ([]*ec2.Subnet, error) {
	subnetsResp, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{VPCFilter(vpcId)},
	})
	if err != nil {
		return nil, err
	}
	return subnetsResp.Subnets, nil
}

func VPCFilter(vpcId string) *ec2.Filter {
	vpcFilter := ec2.Filter{}
	vpcFilter.SetName("vpc-id")
	vpcFilter.SetValues([]*string{&vpcId})
	return &vpcFilter
}

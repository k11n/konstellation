package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/k11n/konstellation/pkg/cloud/types"
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

type EC2Service struct {
	session *session.Session
	svc     *ec2.EC2
}

func NewEC2Service(s *session.Session) *EC2Service {
	return &EC2Service{
		session: s,
		svc:     ec2.New(s),
	}
}

func (s *EC2Service) ListVPCs(ctx context.Context) (vpcs []*types.VPC, err error) {
	vpcResp, err := s.svc.DescribeVpcsWithContext(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return
	}
	for _, vpc := range vpcResp.Vpcs {
		vpcs = append(vpcs, s.toVpcType(vpc))
	}
	return
}

func (s *EC2Service) GetVPC(ctx context.Context, vpcId string) (vpc *types.VPC, err error) {
	resp, err := s.svc.DescribeVpcsWithContext(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []*string{&vpcId},
	})
	if err != nil {
		return
	}

	if len(resp.Vpcs) == 0 {
		err = fmt.Errorf("VPC not found")
		return
	}

	vpc = s.toVpcType(resp.Vpcs[0])
	return
}

func (s *EC2Service) toVpcType(vpc *ec2.Vpc) *types.VPC {
	tVpc := &types.VPC{
		ID:            *vpc.VpcId,
		CloudProvider: "aws",
		CIDRBlock:     *vpc.CidrBlock,
	}
	for _, tag := range vpc.Tags {
		switch *tag.Key {
		case TagVPCTopology:
			tVpc.Topology = *tag.Value
		case TagKonstellation:
			tVpc.SupportsKonstellation = *tag.Value == TagValue1
		}
	}
	return tVpc
}

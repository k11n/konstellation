package aws_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k11n/konstellation/cmd/kon/providers/aws"
)

func TestGetNetworkingOutput(t *testing.T) {
	tf, err := aws.ParseNetworkingTFOutput([]byte(exampleNetworkingOutput))

	assert.NoError(t, err)
	assert.Equal(t, "rtb-0e855a2598de968f8", tf.MainRouteTable)
	assert.Len(t, tf.PrivateSubnets, 2)
	assert.Equal(t, "subnet-039e883c22bdd4532", tf.PrivateSubnets[0].Id)
}

func TestGetClusterOutput(t *testing.T) {
	tf, err := aws.ParseClusterTFOutput([]byte(exampleClusterOutput))

	assert.NoError(t, err)
	assert.Equal(t, "tf2", tf.ClusterName)
	assert.Equal(t, "arn:aws:iam::203125320322:role/kon-tf2-alb-role", tf.AlbIngressRoleArn)
}

const exampleClusterOutput = `
{
  "cluster_alb_role_arn": {
    "sensitive": false,
    "type": "string",
    "value": "arn:aws:iam::203125320322:role/kon-tf2-alb-role"
  },
  "cluster_node_role_arn": {
    "sensitive": false,
    "type": "string",
    "value": "arn:aws:iam::203125320322:role/kon-tf2-node-role"
  },
  "cluster_name": {
    "sensitive": false,
    "type": "string",
    "value": "tf2"
  }
}
`

const exampleNetworkingOutput = `
{
  "main_route_table": {
    "sensitive": false,
    "type": "string",
    "value": "rtb-0e855a2598de968f8"
  },
  "private_subnets": {
    "sensitive": false,
    "type": [
      "tuple",
      [
        [
          "object",
          {
            "arn": "string",
            "assign_ipv6_address_on_creation": "bool",
            "availability_zone": "string",
            "availability_zone_id": "string",
            "cidr_block": "string",
            "id": "string",
            "ipv6_cidr_block": "string",
            "ipv6_cidr_block_association_id": "string",
            "map_public_ip_on_launch": "bool",
            "owner_id": "string",
            "tags": [
              "map",
              "string"
            ],
            "timeouts": [
              "object",
              {
                "create": "string",
                "delete": "string"
              }
            ],
            "vpc_id": "string"
          }
        ],
        [
          "object",
          {
            "arn": "string",
            "assign_ipv6_address_on_creation": "bool",
            "availability_zone": "string",
            "availability_zone_id": "string",
            "cidr_block": "string",
            "id": "string",
            "ipv6_cidr_block": "string",
            "ipv6_cidr_block_association_id": "string",
            "map_public_ip_on_launch": "bool",
            "owner_id": "string",
            "tags": [
              "map",
              "string"
            ],
            "timeouts": [
              "object",
              {
                "create": "string",
                "delete": "string"
              }
            ],
            "vpc_id": "string"
          }
        ]
      ]
    ],
    "value": [
      {
        "arn": "arn:aws:ec2:us-west-2:203125320322:subnet/subnet-039e883c22bdd4532",
        "assign_ipv6_address_on_creation": false,
        "availability_zone": "us-west-2a",
        "availability_zone_id": "usw2-az1",
        "cidr_block": "10.0.16.0/20",
        "id": "subnet-039e883c22bdd4532",
        "ipv6_cidr_block": "",
        "ipv6_cidr_block_association_id": "",
        "map_public_ip_on_launch": false,
        "owner_id": "203125320322",
        "tags": {
          "Konstellation": "1",
          "k11n.dev/az": "us-west-2a",
          "k11n.dev/subnetScope": "private"
        },
        "timeouts": null,
        "vpc_id": "vpc-04d5518c2cc297cb7"
      },
      {
        "arn": "arn:aws:ec2:us-west-2:203125320322:subnet/subnet-0841336dc892354ef",
        "assign_ipv6_address_on_creation": false,
        "availability_zone": "us-west-2b",
        "availability_zone_id": "usw2-az2",
        "cidr_block": "10.0.32.0/20",
        "id": "subnet-0841336dc892354ef",
        "ipv6_cidr_block": "",
        "ipv6_cidr_block_association_id": "",
        "map_public_ip_on_launch": false,
        "owner_id": "203125320322",
        "tags": {
          "Konstellation": "1",
          "k11n.dev/az": "us-west-2b",
          "k11n.dev/subnetScope": "private"
        },
        "timeouts": null,
        "vpc_id": "vpc-04d5518c2cc297cb7"
      }
    ]
  },
  "public_gateway": {
    "sensitive": false,
    "type": "string",
    "value": "igw-08d127debdeec2c14"
  },
  "public_route_table": {
    "sensitive": false,
    "type": "string",
    "value": "rtb-01798a40c829a02ec"
  },
  "public_subnets": {
    "sensitive": false,
    "type": [
      "tuple",
      [
        [
          "object",
          {
            "arn": "string",
            "assign_ipv6_address_on_creation": "bool",
            "availability_zone": "string",
            "availability_zone_id": "string",
            "cidr_block": "string",
            "id": "string",
            "ipv6_cidr_block": "string",
            "ipv6_cidr_block_association_id": "string",
            "map_public_ip_on_launch": "bool",
            "owner_id": "string",
            "tags": [
              "map",
              "string"
            ],
            "timeouts": [
              "object",
              {
                "create": "string",
                "delete": "string"
              }
            ],
            "vpc_id": "string"
          }
        ],
        [
          "object",
          {
            "arn": "string",
            "assign_ipv6_address_on_creation": "bool",
            "availability_zone": "string",
            "availability_zone_id": "string",
            "cidr_block": "string",
            "id": "string",
            "ipv6_cidr_block": "string",
            "ipv6_cidr_block_association_id": "string",
            "map_public_ip_on_launch": "bool",
            "owner_id": "string",
            "tags": [
              "map",
              "string"
            ],
            "timeouts": [
              "object",
              {
                "create": "string",
                "delete": "string"
              }
            ],
            "vpc_id": "string"
          }
        ]
      ]
    ],
    "value": [
      {
        "arn": "arn:aws:ec2:us-west-2:203125320322:subnet/subnet-0a80c85ba256fbb02",
        "assign_ipv6_address_on_creation": false,
        "availability_zone": "us-west-2a",
        "availability_zone_id": "usw2-az1",
        "cidr_block": "10.0.144.0/20",
        "id": "subnet-0a80c85ba256fbb02",
        "ipv6_cidr_block": "",
        "ipv6_cidr_block_association_id": "",
        "map_public_ip_on_launch": false,
        "owner_id": "203125320322",
        "tags": {
          "Konstellation": "1",
          "k11n.dev/az": "us-west-2a",
          "k11n.dev/subnetScope": "private"
        },
        "timeouts": null,
        "vpc_id": "vpc-04d5518c2cc297cb7"
      },
      {
        "arn": "arn:aws:ec2:us-west-2:203125320322:subnet/subnet-04c48eb0c8157e1eb",
        "assign_ipv6_address_on_creation": false,
        "availability_zone": "us-west-2b",
        "availability_zone_id": "usw2-az2",
        "cidr_block": "10.0.160.0/20",
        "id": "subnet-04c48eb0c8157e1eb",
        "ipv6_cidr_block": "",
        "ipv6_cidr_block_association_id": "",
        "map_public_ip_on_launch": false,
        "owner_id": "203125320322",
        "tags": {
          "Konstellation": "1",
          "k11n.dev/az": "us-west-2b",
          "k11n.dev/subnetScope": "private"
        },
        "timeouts": null,
        "vpc_id": "vpc-04d5518c2cc297cb7"
      }
    ]
  },
  "vpc_id": {
    "sensitive": false,
    "type": "string",
    "value": "vpc-04d5518c2cc297cb7"
  }
}`

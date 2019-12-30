package aws

const (
	EKSServicePolicyARN = "arn:aws:iam::aws:policy/AmazonEKSServicePolicy"
	EKSClusterPolicyARN = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"

	EKSWorkerNodePolicy          = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
	EKSCNIPolicy                 = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
	EC2ContainerRegistryROPolicy = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
)

var (
	EKSTrustJSON = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "eks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`
	EC2TrustJSON = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`
)

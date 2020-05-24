variable "region" {
  type = string
  default = "us-west-2"
}

variable "cluster" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "security_group_ids" {
  type = list(string)
}

output "cluster_name" {
  value = aws_eks_cluster.main.id
}

output "cluster_alb_role_arn" {
  value = aws_iam_role.cluster_alb_role.arn
}

output "cluster_node_role_arn" {
  value = aws_iam_role.eks_node_role.arn
}

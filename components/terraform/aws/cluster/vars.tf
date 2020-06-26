variable "region" {
  type = string
}

variable "cluster" {
  type = string
}

variable "vpc_id" {
  type = string
  default = ""
}

variable "security_group_ids" {
  type = list(string)
  default = []
}

variable "kube_version" {
  type = string
  default = ""
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

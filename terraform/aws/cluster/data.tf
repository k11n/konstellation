data "aws_iam_role" "eks_service_role" {
  name = "kon-eks-service-role"
}

data "aws_subnet_ids" "selected" {
  vpc_id = var.vpc_id

  tags = local.common_tags
}

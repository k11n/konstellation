locals {
  cluster_tags = merge(
    local.common_tags,
    {
      "k11n.dev/clusterName" = var.cluster
    }
  )
}

resource "aws_eks_cluster" "main" {
  name = var.cluster
  role_arn = aws_iam_role.eks_service_role.arn

  version = var.kube_version
  vpc_config {
    subnet_ids = data.aws_subnet_ids.selected.ids
    security_group_ids = var.security_group_ids
    endpoint_private_access = true
    endpoint_public_access = true
  }

  tags = local.common_tags
}

// OIDC provider
resource "aws_iam_openid_connect_provider" "main" {
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = []
  url             = aws_eks_cluster.main.identity.0.oidc.0.issuer
}

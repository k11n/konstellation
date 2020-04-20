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
  role_arn = data.aws_iam_role.eks_service_role.arn

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

data "aws_caller_identity" "current" {}

data "aws_iam_policy_document" "assume_role_policy" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = "${replace(aws_iam_openid_connect_provider.main.url, "https://", "")}:sub"
      values   = ["system:serviceaccount:kube-system:alb-ingress-controller"]
    }

    principals {
      identifiers = [aws_iam_openid_connect_provider.main.arn]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "cluster_alb_role" {
  name               = "kon-alb-role-${var.cluster}"
  assume_role_policy = data.aws_iam_policy_document.assume_role_policy.json

  tags = local.cluster_tags
}

// associate ingress policy with it
resource "aws_iam_role_policy_attachment" "alb_ingress_attachment" {
  role = aws_iam_role.cluster_alb_role.name
  policy_arn = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:policy/ALBIngressControllerIAMPolicy"
}

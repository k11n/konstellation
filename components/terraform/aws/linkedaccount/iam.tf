// Role role
data "aws_iam_policy_document" "assume_role_policy" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = "${var.oidc_url}:sub"
      // [for value in aws_route_table.private: value.id]
      values   = [for target in var.targets:
        "system:serviceaccount:${target}:${var.account}"
      ]
    }

    principals {
      identifiers = [var.oidc_arn]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "cluster_account_role" {
  name               = "kon-${var.cluster}-${var.account}"
  assume_role_policy = data.aws_iam_policy_document.assume_role_policy.json

  tags = local.common_tags
}

// attach each policy to the role
resource "aws_iam_role_policy_attachment" "policy_attachment" {
  for_each = toset(var.policies)

  role = aws_iam_role.cluster_account_role.name
  policy_arn = each.value
}
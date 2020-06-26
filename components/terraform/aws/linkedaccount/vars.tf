variable "region" {
  # Not needed, declared for consistency with other actions
  type = string
}

variable "cluster" {
  type = string
}

variable "account" {
  type = string
}

variable "targets" {
  type = list(string)
  default = []
}

variable "policies" {
  type = list(string)
  default = []
}

variable "oidc_url" {
  type = string
  default = ""
}

variable "oidc_arn" {
  type = string
  default = ""
}

output "role_arn" {
  value = aws_iam_role.cluster_account_role.arn
}

variable "region" {
  type = string
}

variable "cluster" {
  type = string
}

variable "nodepool" {
  type = string
}

variable "ami_type" {
  type = string
  default = ""
}

variable "disk_size" {
  type = number
  default = 20
}

variable "instance_type" {
  type = string
  default = ""
}

variable "role_arn" {
  type = string
  default = ""
}


variable "desired_size" {
  type = number
  default = 0
}

variable "min_size" {
  type = number
  default = 0
}

variable "max_size" {
  type = number
  default = 0
}

variable "subnet_ids" {
  type = list(string)
  default = []
}

variable "enable_remote_access" {
  type = bool
  default = false
}

// used only if enable_remote_access is true
variable "ec2_ssh_key" {
  type = string
  default = ""
}

// used only if enable_remote_access is true
variable "source_security_group_ids" {
  type = list(string)
  default = []
}

// output "role_arn" {
//   value = aws_iam_role.cluster_account_role.arn
// }

output "asg_id" {
  // TODO
}
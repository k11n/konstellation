variable "vpc_cidr" {
  type = string
  default = "10.1.0.0/16"
}

variable "az_suffixes" {
  type = list(string)
  default = [
    "a",
    "b",
  ]
}

variable "enable_ipv6" {
  type = bool
  default = false
}

variable "region" {
  type = string
  default = "us-west-2"
}

variable "az_number" {
  # Assign a number to each AZ letter used in our configuration
  default = {
    a = 1
    b = 2
    c = 3
    d = 4
    e = 5
    f = 6
  }
}

variable "topology" {
  type = string
  default = "public"
}

output "vpc_id" {
  value = aws_vpc.main.id
}

output "ipv6_cidr" {
  value = aws_vpc.main.ipv6_cidr_block
}

output "public_subnets" {
  value = values(aws_subnet.public)
}

output "main_route_table" {
  value = aws_vpc.main.main_route_table_id
}

output "public_gateway" {
  value = aws_internet_gateway.public_gw.id
}
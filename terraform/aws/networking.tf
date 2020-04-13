locals {
  public_cidr = cidrsubnet(var.vpc_cidr, 1, 0)
}

output "vpc_id" {
  value = aws_vpc.main.id
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

// VPC
resource "aws_vpc" "main" {
  cidr_block = var.vpc_cidr

  tags = local.common_tags
}


// create public subnets
resource "aws_subnet" "public" {
  for_each = toset(var.az_suffixes)

  vpc_id = aws_vpc.main.id
  availability_zone = "${var.region}${each.key}"
  cidr_block = cidrsubnet(local.public_cidr, 3, var.az_number[each.key])
  map_public_ip_on_launch = true

  tags = merge(
    local.common_tags,
    {
      "k11n.dev/subnetScope" = "private",
      "k11n.dev/az" = "${var.region}${each.key}",
      "kubernetes.io/role/elb" = "1",
    }
  )
  depends_on = [aws_vpc.main]
}

resource "aws_internet_gateway" "public_gw" {
  vpc_id = aws_vpc.main.id

  tags = local.common_tags
  depends_on = [aws_vpc.main]
}

resource "aws_route" "gw_route" {
  route_table_id = aws_vpc.main.main_route_table_id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id = aws_internet_gateway.public_gw.id

  depends_on = [aws_vpc.main, aws_internet_gateway.public_gw]
}
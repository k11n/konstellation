locals {
  public_cidr = cidrsubnet(var.vpc_cidr, 1, 0)
}

// VPC
resource "aws_vpc" "main" {
  cidr_block = var.vpc_cidr
  enable_dns_support = true
  enable_dns_hostnames = true
  assign_generated_ipv6_cidr_block = var.enable_ipv6

  tags = merge(
    local.common_tags,
    {
      "k11n.dev/topology" = var.topology,
    }
  )
}


// create public subnets
resource "aws_subnet" "public" {
  for_each = toset(var.az_suffixes)

  vpc_id = aws_vpc.main.id
  availability_zone = "${var.region}${each.key}"
  cidr_block = cidrsubnet(local.public_cidr, 3, var.az_number[each.key])
  map_public_ip_on_launch = true
  ipv6_cidr_block = var.enable_ipv6 ? cidrsubnet(aws_vpc.main.ipv6_cidr_block, 8, var.az_number[each.key]) : null
  assign_ipv6_address_on_creation = var.enable_ipv6

  tags = merge(
    local.common_tags,
    {
      "Name" = "kon-public-${each.key}"
      "k11n.dev/subnetScope" = "public",
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

resource "aws_route" "gw_route_ipv4" {
  route_table_id = aws_vpc.main.main_route_table_id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id = aws_internet_gateway.public_gw.id

  depends_on = [aws_vpc.main, aws_internet_gateway.public_gw]
}

resource "aws_route" "gw_route_ipv6" {
  count = var.enable_ipv6 ? 1 : 0

  route_table_id = aws_vpc.main.main_route_table_id
  destination_ipv6_cidr_block = "::/0"
  gateway_id = aws_internet_gateway.public_gw.id

  depends_on = [aws_vpc.main, aws_internet_gateway.public_gw]
}

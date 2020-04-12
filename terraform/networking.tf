locals {
  public_cidr = cidrsubnet(var.vpc_cidr, 1, 0)
  private_cidr = cidrsubnet(var.vpc_cidr, 1, 1)
}

output "vpc_id" {
  value = aws_vpc.main.id
}

output "private_subnets" {
  value = aws_subnet.private
}

output "public_subnets" {
  value = aws_subnet.public
}

output "main_route_table" {
  value = aws_vpc.main.main_route_table_id
}

output "public_route_table" {
  value = aws_route_table.public.id
}

output "public_gateway" {
  value = aws_internet_gateway.public_gw.id
}


// VPC
resource "aws_vpc" "main" {
  cidr_block = var.vpc_cidr

  tags = local.common_tags
}


// private network
resource "aws_subnet" "private" {
  for_each = toset(var.az_suffixes)

  vpc_id = aws_vpc.main.id
  availability_zone = "${var.region}${each.key}"
  cidr_block = cidrsubnet(local.private_cidr, 3, var.az_number[each.key])

  tags = merge(
    local.common_tags,
    {
      "k11n.dev/subnetScope" = "private",
      "k11n.dev/az" = "${var.region}${each.key}"
    }
  )

  depends_on = [aws_vpc.main]
}

resource "aws_eip" "nat" {
  for_each = aws_subnet.private
  vpc = true
  tags = merge(
    local.common_tags,
    {
      "k11n.dev/purpose" = "nat_gateway",
      "k11n.dev/subnet" = each.value.id,
    }
  )

  depends_on = [aws_subnet.private]
}

data "aws_subnet_ids" "private" {
  vpc_id = aws_vpc.main.id
  tags = merge(
    local.common_tags,
    { "k11n.dev/subnetScope" = "private" }
  )

  depends_on = [aws_subnet.private]
}

resource "aws_nat_gateway" "private_gw" {
  for_each = aws_eip.nat
  allocation_id = each.value.id
  subnet_id = each.value.tags["k11n.dev/subnet"]

  tags = local.common_tags
  depends_on = [aws_eip.nat, aws_subnet.private]
}


// create public subnets
resource "aws_subnet" "public" {
  for_each = toset(var.az_suffixes)

  vpc_id = aws_vpc.main.id
  availability_zone = "${var.region}${each.key}"
  cidr_block = cidrsubnet(local.public_cidr, 3, var.az_number[each.key])

  tags = merge(
    local.common_tags,
    {
      "k11n.dev/subnetScope" = "private",
      "k11n.dev/az" = "${var.region}${each.key}"
    }
  )
  depends_on = [aws_vpc.main]
}

resource "aws_internet_gateway" "public_gw" {
  vpc_id = aws_vpc.main.id

  tags = local.common_tags
  depends_on = [aws_vpc.main]
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.public_gw.id
  }
  tags = local.common_tags
  depends_on = [aws_vpc.main, aws_internet_gateway.public_gw]
}

resource "aws_route_table_association" "public" {
  for_each = aws_subnet.public
  subnet_id = each.value.id
  route_table_id = aws_route_table.public.id
  depends_on = [aws_route_table.public, aws_subnet.public]
}
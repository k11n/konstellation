locals {
  private_cidr = cidrsubnet(var.vpc_cidr, 1, 1)
}

output "private_subnets" {
  value = values(aws_subnet.private)
}

output "private_route_tables" {
  value = [for value in aws_route_table.private: value.id]
}

resource "aws_subnet" "private" {
  for_each = toset(var.az_suffixes)

  vpc_id = aws_vpc.main.id
  availability_zone = "${var.region}${each.key}"
  cidr_block = cidrsubnet(local.private_cidr, 3, var.az_number[each.key])

  tags = merge(
    local.common_tags,
    {
      "k11n.dev/subnetScope" = "private",
      "k11n.dev/az" = "${var.region}${each.key}",
      "kubernetes.io/role/internal-elb" = "1"
    }
  )

  depends_on = [aws_vpc.main]
}

resource "aws_eip" "nat" {
  // one for each public nat gateway, which is one for each subnet
  for_each = aws_subnet.public
  vpc = true
  tags = merge(
    local.common_tags,
    {
      "k11n.dev/purpose" = "nat_gateway",
      "k11n.dev/publicSubnet" = each.value.id,
    }
  )

  depends_on = [aws_subnet.public]
}

resource "aws_nat_gateway" "private_gw" {
  for_each = aws_eip.nat
  allocation_id = each.value.id
  subnet_id = each.value.tags["k11n.dev/publicSubnet"]

  tags = merge(
    local.common_tags,
    {
      "k11n.dev/publicSubnet" = each.value.tags["k11n.dev/publicSubnet"],
      "k11n.dev/privateSubnet" = aws_subnet.private[each.key].id
    }
  )
  depends_on = [aws_eip.nat, aws_subnet.private]
}

resource "aws_route_table" "private" {
  for_each = aws_nat_gateway.private_gw
  vpc_id = aws_vpc.main.id

  tags = merge(
    local.common_tags,
    {
      "k11n.dev/subnet" = each.value.tags["k11n.dev/privateSubnet"]
      "k11n.dev/natGateway" = each.value.id
    }
  )
  depends_on = [aws_nat_gateway.private_gw]
}

resource "aws_route" "nat_gateway" {
  for_each = aws_route_table.private

  // route_table_id = aws_route_table.private.id
  route_table_id = each.value.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id = each.value.tags["k11n.dev/natGateway"]

  depends_on = [aws_route_table.private]
}

resource "aws_route_table_association" "private" {
  for_each = aws_route_table.private

  subnet_id = each.value.tags["k11n.dev/subnet"]
  route_table_id = each.value.id
  depends_on = [aws_route_table.private, aws_subnet.private]
}

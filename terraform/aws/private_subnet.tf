locals {
  private_cidr = cidrsubnet(var.vpc_cidr, 1, 1)
}

output "private_subnets" {
  value = values(aws_subnet.private)
}

output "private_route_table" {
  value = aws_route_table.private.id
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

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  tags = local.common_tags
  depends_on = [aws_vpc.main]
}

resource "aws_route_table_association" "private" {
  for_each = aws_subnet.private
  subnet_id = each.value.id
  route_table_id = aws_route_table.private.id
  depends_on = [aws_route_table.private, aws_subnet.private]
}
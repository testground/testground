resource "aws_vpc" "vpc" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = merge(
    var.default_tags,
    {
      "Name"                                      = var.cluster-name
      "kubernetes.io/cluster/${var.cluster-name}" = "shared"
    },
  )
}

resource "aws_subnet" "subnet" {
  count = 2

  availability_zone = data.aws_availability_zones.available.names[count.index]
  cidr_block        = "10.0.${64 * count.index}.0/18"
  vpc_id            = aws_vpc.vpc.id

  tags = merge(
    var.default_tags,
    {
      "Name"                                      = var.cluster-name
      "kubernetes.io/cluster/${var.cluster-name}" = "shared"
    },
  )
}

resource "aws_internet_gateway" "ig" {
  vpc_id = aws_vpc.vpc.id

  tags = merge(
    var.default_tags,
    {
      "Name" = var.cluster-name
    },
  )
}

resource "aws_route_table" "rt" {
  vpc_id = aws_vpc.vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.ig.id
  }

  tags = merge(var.default_tags)
}

resource "aws_route_table_association" "rta" {
  count = 2

  subnet_id      = aws_subnet.subnet[count.index].id
  route_table_id = aws_route_table.rt.id
}

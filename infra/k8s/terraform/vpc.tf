resource "aws_vpc" "testground" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    "Name"                                      = "${var.cluster-name}-node"
    "kubernetes.io/cluster/${var.cluster-name}" = "shared"
  }
}

resource "aws_subnet" "testground" {
  count = 2

  availability_zone = data.aws_availability_zones.available.names[count.index]
  cidr_block        = "10.0.${count.index}.0/24"
  vpc_id            = aws_vpc.testground.id

  tags = {
    "Name"                                      = "${var.cluster-name}-node"
    "kubernetes.io/cluster/${var.cluster-name}" = "shared"
  }
}

resource "aws_internet_gateway" "testground" {
  vpc_id = aws_vpc.testground.id

  tags = {
    Name = var.cluster-name
  }
}

resource "aws_route_table" "testground" {
  vpc_id = aws_vpc.testground.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.testground.id
  }
}

resource "aws_route_table_association" "testground" {
  count = 2

  subnet_id      = aws_subnet.testground[count.index].id
  route_table_id = aws_route_table.testground.id
}


resource "aws_security_group" "frontend" {
  name        = "${var.name_prefix}-frontend-sg"
  description = "Public ingress for frontend edge/service endpoints"
  vpc_id      = var.vpc_id

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "Allow all outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-frontend-sg"
  })
}

resource "aws_security_group" "api" {
  name        = "${var.name_prefix}-api-sg"
  description = "Ingress for API services from frontend"
  vpc_id      = var.vpc_id

  ingress {
    description     = "API from frontend"
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.frontend.id]
  }

  egress {
    description = "Allow all outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-api-sg"
  })
}

resource "aws_security_group" "internal" {
  name        = "${var.name_prefix}-internal-sg"
  description = "Internal-only services and metadata tier"
  vpc_id      = var.vpc_id

  ingress {
    description     = "Internal traffic from API"
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    security_groups = [aws_security_group.api.id]
  }

  egress {
    description = "Allow all outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-internal-sg"
  })
}

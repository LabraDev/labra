locals {
  service_subnet_ids     = var.assign_public_ip ? var.public_subnet_ids : (length(var.private_subnet_ids) > 0 ? var.private_subnet_ids : var.public_subnet_ids)
  api_efs_file_system_id = trimspace(coalesce(var.api_efs_file_system_id, ""))
  enable_api_efs         = local.api_efs_file_system_id != ""

  services = {
    "control-api" = {
      image         = trimspace(var.api_container_image)
      desired_count = var.api_desired_count
      port          = var.api_container_port
      lb_enabled    = true
      sg_id         = var.api_service_security_group_id
      environment   = lookup(var.service_environment, "control-api", {})
    }
    "deploy-orchestrator" = {
      image         = trimspace(var.deploy_orchestrator_container_image)
      desired_count = var.worker_desired_count
      port          = null
      lb_enabled    = false
      sg_id         = var.worker_service_security_group_id
      environment   = lookup(var.service_environment, "deploy-orchestrator", {})
    }
    "webhook-ingestor" = {
      image         = trimspace(var.webhook_ingestor_container_image)
      desired_count = var.worker_desired_count
      port          = null
      lb_enabled    = false
      sg_id         = var.worker_service_security_group_id
      environment   = lookup(var.service_environment, "webhook-ingestor", {})
    }
  }

  resolved_execution_role_arn = var.create_execution_role ? aws_iam_role.task_execution[0].arn : trimspace(coalesce(var.execution_role_arn, ""))
}

resource "aws_lb" "api" {
  name               = substr("${var.name_prefix}-api-alb", 0, 32)
  load_balancer_type = "application"
  internal           = false
  security_groups    = [var.alb_security_group_id]
  subnets            = var.public_subnet_ids

  tags = merge(var.tags, {
    Component = "control-api-alb"
  })
}

resource "aws_lb_target_group" "api" {
  name        = substr("${var.name_prefix}-api-tg", 0, 32)
  port        = var.api_container_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = var.vpc_id

  health_check {
    path                = var.api_health_check_path
    matcher             = "200-499"
    healthy_threshold   = 2
    unhealthy_threshold = 2
    interval            = 30
    timeout             = 6
  }

  tags = merge(var.tags, {
    Component = "control-api-target-group"
  })
}

resource "aws_lb_listener" "api_http" {
  load_balancer_arn = aws_lb.api.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }
}

data "aws_iam_policy_document" "task_execution_assume_role" {
  count = var.create_execution_role ? 1 : 0

  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "task_execution" {
  count = var.create_execution_role ? 1 : 0

  name               = "${var.name_prefix}-ecs-task-execution-role"
  assume_role_policy = data.aws_iam_policy_document.task_execution_assume_role[0].json

  tags = merge(var.tags, {
    Component = "ecs-task-execution"
  })
}

resource "aws_iam_role_policy_attachment" "task_execution_managed" {
  count = var.create_execution_role ? 1 : 0

  role       = aws_iam_role.task_execution[0].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_ecs_task_definition" "service" {
  for_each = local.services

  family                   = "${var.name_prefix}-${each.key}"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = tostring(var.task_cpu)
  memory                   = tostring(var.task_memory)
  execution_role_arn       = local.resolved_execution_role_arn
  task_role_arn            = trimspace(lookup(var.task_role_arns, each.key, "")) != "" ? trimspace(lookup(var.task_role_arns, each.key, "")) : null

  dynamic "volume" {
    for_each = each.key == "control-api" && local.enable_api_efs ? [1] : []

    content {
      name = "control-api-db"

      efs_volume_configuration {
        file_system_id     = local.api_efs_file_system_id
        root_directory     = "/"
        transit_encryption = "ENABLED"
      }
    }
  }

  container_definitions = jsonencode([
    merge(
      {
        name      = each.key
        image     = each.value.image
        essential = true
        logConfiguration = {
          logDriver = "awslogs"
          options = {
            awslogs-group         = "/aws/labra/${var.name_prefix}/${each.key}"
            awslogs-region        = data.aws_region.current.name
            awslogs-stream-prefix = "ecs"
          }
        },
        environment = [
          for env_key, env_value in each.value.environment : {
            name  = env_key
            value = env_value
          }
        ]
      },
      each.key == "control-api" ? {
        portMappings = [
          {
            containerPort = each.value.port
            hostPort      = each.value.port
            protocol      = "tcp"
          }
        ]
      } : {},
      each.key == "control-api" && local.enable_api_efs ? {
        mountPoints = [
          {
            sourceVolume  = "control-api-db"
            containerPath = var.api_db_mount_path
            readOnly      = false
          }
        ]
      } : {},
      each.key != "control-api" ? {
        command = ["sh", "-c", "while true; do sleep 300; done"]
      } : {}
    )
  ])

  tags = merge(var.tags, {
    Component = each.key
  })
}

resource "aws_ecs_service" "service" {
  for_each = local.services

  name            = "${var.name_prefix}-${each.key}"
  cluster         = var.cluster_arn
  task_definition = aws_ecs_task_definition.service[each.key].arn
  desired_count   = each.value.desired_count
  launch_type     = "FARGATE"

  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200

  network_configuration {
    subnets          = local.service_subnet_ids
    security_groups  = [each.value.sg_id]
    assign_public_ip = var.assign_public_ip
  }

  dynamic "load_balancer" {
    for_each = each.value.lb_enabled ? [1] : []

    content {
      target_group_arn = aws_lb_target_group.api.arn
      container_name   = each.key
      container_port   = each.value.port
    }
  }

  depends_on = [aws_lb_listener.api_http]

  tags = merge(var.tags, {
    Component = each.key
  })
}

data "aws_region" "current" {}

variable "name_prefix" {
  type = string
}

variable "cluster_arn" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "public_subnet_ids" {
  type = list(string)
}

variable "private_subnet_ids" {
  type    = list(string)
  default = []
}

variable "alb_security_group_id" {
  type = string
}

variable "api_service_security_group_id" {
  type = string
}

variable "worker_service_security_group_id" {
  type = string
}

variable "api_container_image" {
  type    = string
  default = "public.ecr.aws/nginx/nginx:stable"
}

variable "deploy_orchestrator_container_image" {
  type    = string
  default = "public.ecr.aws/docker/library/busybox:stable"
}

variable "webhook_ingestor_container_image" {
  type    = string
  default = "public.ecr.aws/docker/library/busybox:stable"
}

variable "api_container_port" {
  type    = number
  default = 8080
}

variable "api_health_check_path" {
  type    = string
  default = "/health"
}

variable "api_desired_count" {
  type    = number
  default = 1
}

variable "worker_desired_count" {
  type    = number
  default = 1
}

variable "task_cpu" {
  type    = number
  default = 256
}

variable "task_memory" {
  type    = number
  default = 512
}

variable "assign_public_ip" {
  type    = bool
  default = false
}

variable "create_execution_role" {
  type    = bool
  default = true
}

variable "execution_role_arn" {
  type    = string
  default = null

  validation {
    condition     = var.create_execution_role || trimspace(var.execution_role_arn == null ? "" : var.execution_role_arn) != ""
    error_message = "execution_role_arn must be set when create_execution_role is false."
  }
}

variable "task_role_arns" {
  type    = map(string)
  default = {}
}

variable "service_environment" {
  type    = map(map(string))
  default = {}
}

variable "service_secrets" {
  type    = map(map(string))
  default = {}
}

variable "secret_read_arns" {
  type    = list(string)
  default = []
}

variable "api_efs_file_system_id" {
  type    = string
  default = null
}

variable "api_db_mount_path" {
  type    = string
  default = "/mnt/labra-db"
}

variable "tags" {
  type    = map(string)
  default = {}
}

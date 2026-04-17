variable "name_prefix" {
  type = string
}

variable "subnet_ids" {
  type = list(string)
}

variable "security_group_id" {
  type = string
}

variable "performance_mode" {
  type    = string
  default = "generalPurpose"

  validation {
    condition     = contains(["generalPurpose", "maxIO"], var.performance_mode)
    error_message = "performance_mode must be generalPurpose or maxIO."
  }
}

variable "throughput_mode" {
  type    = string
  default = "bursting"

  validation {
    condition     = contains(["bursting", "provisioned", "elastic"], var.throughput_mode)
    error_message = "throughput_mode must be bursting, provisioned, or elastic."
  }
}

variable "provisioned_throughput_mibps" {
  type    = number
  default = null
}

variable "transition_to_ia" {
  type    = string
  default = "AFTER_30_DAYS"

  validation {
    condition = contains([
      "AFTER_7_DAYS",
      "AFTER_14_DAYS",
      "AFTER_30_DAYS",
      "AFTER_60_DAYS",
      "AFTER_90_DAYS",
      "AFTER_1_DAY"
    ], var.transition_to_ia)
    error_message = "transition_to_ia must be a valid EFS transition value."
  }
}

variable "tags" {
  type    = map(string)
  default = {}
}

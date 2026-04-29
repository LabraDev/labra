variable "name_prefix" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "create_platform_secret" {
  type    = bool
  default = true
}

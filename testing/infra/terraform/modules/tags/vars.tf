variable "project" {
  description = "The value to use for the project tag/label"
  type        = string
}

variable "build" {
  description = "The value to use for the build tag/label"
  type        = string
  default     = "unknown"
}

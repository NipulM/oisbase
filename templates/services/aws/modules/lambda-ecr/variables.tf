variable "function_name" {
  description = "Name of the Lambda function"
  type        = string
}

variable "iam_role_arn" {
  description = "IAM Role ARN for Lambda execution"
  type        = string
}

variable "timeout" {
  description = "Execution timeout for the function"
  type        = number
  default     = 30
}


variable "memory_size" {
  description = "Memory size for Lambda function"
  type        = number
  default     = 128
}

variable "environment" {
  description = "Environment (e.g., dev, stg, prod)"
  type        = string
}

variable "environment_variables" {
  description = "Map of environment variables for Lambda function"
  type        = map(string)
  default     = {}
}

variable "subnet_ids" {
  description = "List of subnet IDs for the Lambda function"
  type        = list(string)
  default     = []
}

variable "security_group_ids" {
  description = "List of security group IDs for the Lambda function"
  type        = list(string)
  default     = []
}

variable "log_retention_in_days" {
  description = "Execution timeout for the function"
  type        = number
  default     = 30
}

variable "layers" {
  description = "List of Lambda layer ARNs to attach to the function"
  type        = list(string)
  default     = []
}

variable "image_uri" {
  description = "URI of the Docker image for the Lambda function"
  type        = string
}
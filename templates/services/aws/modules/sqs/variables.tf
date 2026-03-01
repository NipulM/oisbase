variable "environment" {
  description = "Deployment environment (e.g., dev, prod)"
  type        = string
  validation {
    condition     = contains(["dev", "stg", "prod"], var.environment)
    error_message = "The environment must be one of: dev, stg, prod."
  }
}

variable "queue_policy_statements" {
  description = "Map of named SQS queue policy statements. Each statement defines permissions for sending messages to the queue, typically for allowing SNS topics or other AWS services."
  type = map(object({
    sid     = string
    actions = list(string)
    principals = list(object({
      type        = string
      identifiers = list(string)
    }))
    conditions = optional(list(object({
      test     = string
      variable = string
      values   = list(string)
    })), [])
  }))
  default = {}
}

variable "fifo_queue" {
  description = "Indicates whether the queue is a FIFO queue"
  type        = bool
  default     = false
}

variable "create_dlq" {
  description = "Whether to create a dead-letter queue (DLQ)"
  type        = bool
  default     = true
}

variable "queue_name" {
  description = "The name of the SQS queue"
  type        = string
}


variable "tags" {
  description = "Additional tags to apply to the resources"
  type        = map(string)
  default     = {}
}

variable "delay_seconds" {
  description = "Delay in seconds for the queue"
  type        = number
  default     = 0
}

variable "max_message_size" {
  description = "Maximum message size in bytes"
  type        = number
  default     = 262144
}

variable "message_retention_seconds" {
  description = "Message retention in seconds"
  type        = number
  default     = 345600
}

variable "receive_wait_time_seconds" {
  description = "Receive wait time in seconds"
  type        = number
  default     = 20
}

variable "visibility_timeout_seconds" {
  description = "Visibility timeout in seconds"
  type        = number
  default     = 60
}

variable "max_receive_count" {
  description = "How many times a message can be received before moving to DLQ"
  type        = number
  default     = 5
}
variable "db_name" {
  type = string
}

variable "billing_mode" {
  type        = string
  description = "Controls how you are billed for read/write throughput and how you manage capacity. The valid values are PROVISIONED or PAY_PER_REQUEST"
}

variable "read_capacity" {
  description = "Read capacity units (only for PROVISIONED billing mode)"
  type        = number
  default     = 5
}

variable "write_capacity" {
  description = "Write capacity units (only for PROVISIONED billing mode)"
  type        = number
  default     = 5
}

variable "attributes" {
  type = list(object({
    name = string
    type = string
  }))
}

variable "hash_key" {
  type        = string
  description = "DynamoDB hash_key"
}

variable "range_key" {
  type        = string
  description = "DynamoDB range_key"
}

variable "global_secondary_indexes" {
  type = list(object({
    name               = string
    hash_key           = string
    range_key          = string
    projection_type    = string
    non_key_attributes = list(string)
  }))
  default = []
}

variable "additional_tags" {
  default     = {}
  description = "Additional resource tags"
  type        = map(string)
}
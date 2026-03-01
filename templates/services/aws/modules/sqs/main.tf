module "sqs" {
  source = "terraform-aws-modules/sqs/aws"

  name = var.queue_name

  fifo_queue                 = var.fifo_queue
  delay_seconds              = var.delay_seconds
  max_message_size           = var.max_message_size
  message_retention_seconds  = var.message_retention_seconds
  receive_wait_time_seconds  = var.receive_wait_time_seconds
  visibility_timeout_seconds = var.visibility_timeout_seconds

  create_dlq = var.create_dlq
  redrive_policy = {
    maxReceiveCount = var.max_receive_count
  }

  tags = merge(
    { Environment = var.environment },
    var.tags
  )
}

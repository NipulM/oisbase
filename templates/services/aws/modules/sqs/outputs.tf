output "queue_url" {
  value = module.sqs.queue_url
}

output "queue_arn" {
  value = module.sqs.queue_arn
}

output "dlq_arn" {
  value = module.sqs.dead_letter_queue_arn
}

output "dlq_url" {
  value = module.sqs.dead_letter_queue_url
}
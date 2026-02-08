resource "aws_lambda_function" "lambda" {
  function_name = var.function_name
  role          = var.iam_role_arn
  handler       = var.handler
  runtime       = var.runtime
  timeout       = var.timeout
  memory_size   = var.memory_size
  layers        = var.layers

  s3_bucket = var.s3_bucket
  s3_key    = var.s3_key

  lifecycle {
    ignore_changes        = [s3_bucket, s3_key]
    create_before_destroy = true
  }

  environment {
    variables = merge(var.environment_variables)
  }

  tags = {
    Name        = var.function_name
    Environment = var.environment
  }
}

resource "aws_cloudwatch_log_group" "lambda_log_group" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = var.log_retention_in_days
  tags = {
    Name        = var.function_name
    Environment = var.environment
  }
}
resource "aws_lambda_function" "lambda" {
  function_name = var.function_name
  role          = var.iam_role_arn
  timeout       = var.timeout
  memory_size   = var.memory_size
  layers        = var.layers

  package_type = "Image"
  image_uri    = var.image_uri

  environment {
    variables = var.environment_variables
  }

  vpc_config {
    subnet_ids         = sort(var.subnet_ids)
    security_group_ids = sort(var.security_group_ids)
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
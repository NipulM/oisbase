module "dynamodb_table" {
  source = "terraform-aws-modules/dynamodb-table/aws"

  billing_mode = var.billing_mode

  read_capacity  = var.billing_mode == "PROVISIONED" ? var.read_capacity : null
  write_capacity = var.billing_mode == "PROVISIONED" ? var.write_capacity : null

  name       = var.db_name
  attributes = var.attributes
  hash_key   = var.hash_key
  range_key  = var.range_key

  tags = merge(
    var.additional_tags,
    {
      "db_name" = "${var.db_name}"
    },
  )
}

resource "aws_cloudwatch_log_group" "lambda" {
  for_each = local.lambdas

  name              = "/aws/lambda/${local.name}-${each.key}"
  retention_in_days = 7
}

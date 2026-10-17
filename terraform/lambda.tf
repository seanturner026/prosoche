resource "aws_lambda_function" "lambda" {
  for_each = local.lambdas

  function_name    = "${local.name}-${each.key}"
  description      = each.value.description
  role             = aws_iam_role.lambda[each.key].arn
  filename         = data.archive_file.lambda[each.key].output_path
  source_code_hash = data.archive_file.lambda[each.key].output_base64sha256
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  timeout          = 30

  dynamic "environment" {
    for_each = length(each.value.env_vars) > 0 ? [1] : []

    content {
      variables = each.value.env_vars
    }
  }
}

resource "aws_iam_role" "lambda" {
  for_each = local.lambdas

  name               = "${local.name}-${each.key}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

resource "aws_iam_role_policy_attachment" "lambda_basic" {
  for_each = local.lambdas

  role       = aws_iam_role.lambda[each.key].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "ssm" {
  for_each = local.lambdas

  name   = "ssm"
  role   = aws_iam_role.lambda[each.key].name
  policy = data.aws_iam_policy_document.lambda_ssm.json
}

resource "aws_iam_role" "scheduler" {
  name               = "${local.name}-scheduler"
  assume_role_policy = data.aws_iam_policy_document.scheduler_assume_role.json
}

resource "aws_iam_role_policy" "scheduler_invoke" {
  name   = "invoke"
  role   = aws_iam_role.scheduler.name
  policy = data.aws_iam_policy_document.scheduler_invoke.json
}

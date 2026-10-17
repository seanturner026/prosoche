resource "aws_scheduler_schedule" "daily" {
  for_each = var.schedules

  name                         = "${local.name}-${each.key}"
  schedule_expression          = each.value
  schedule_expression_timezone = var.timezone

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = aws_lambda_function.lambda["daily"].arn
    role_arn = aws_iam_role.scheduler.arn
  }
}

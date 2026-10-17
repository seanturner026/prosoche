variable "discord_channel_id" {
  description = "Discord channel ID to post the daily passage — from channel settings"
  type        = string
  default     = "1561019950376681542"
}

variable "discord_ssm_parameter" {
  description = "captain-hook's SSM parameter; only its bot_token key is read"
  type        = string
  default     = "/captain-hook-production/discord"
}

variable "schedules" {
  description = "EventBridge Scheduler cron expressions, evaluated in var.timezone"
  type        = map(string)
  default = {
    weekday = "cron(40 7 ? * MON-FRI *)"
    weekend = "cron(0 11 ? * SAT,SUN *)"
  }
}

variable "timezone" {
  description = "IANA time zone for var.schedules"
  type        = string
  default     = "America/New_York"
}

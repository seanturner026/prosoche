variable "discord_channel_id" {
  description = "Discord channel ID to post the daily passage — from channel settings"
  type        = string
  ephemeral   = true
}

variable "discord_bot_token" {
  description = "Discord bot token — from discord.dev Bot tab"
  type        = string
  ephemeral   = true
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

variable "tier" {
  description = "Deployment tier — must match the Terraform workspace name"
  type        = string

  validation {
    condition     = contains(["staging", "production"], var.tier)
    error_message = "tier must be 'staging' or 'production'."
  }
}

locals {
  name = "prosoche"

  lambdas = {
    daily = {
      description = "Posts the day's Stoic passage to Discord"
      env_vars = {
        DISCORD_CHANNEL_ID = var.discord_channel_id
        SSM_PARAMETER      = var.discord_ssm_parameter
      }
    }
  }
}

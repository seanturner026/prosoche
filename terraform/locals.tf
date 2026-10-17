locals {
  tier = terraform.workspace
  name = "prosoche-${local.tier}"

  lambdas = {
    daily = {
      description = "Posts the day's Stoic passage to Discord"
      env_vars = {
        SSM_PARAMETER = "/${local.name}/discord"
      }
    }
  }
}

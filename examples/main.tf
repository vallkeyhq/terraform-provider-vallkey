terraform {
  required_providers {
    vallkey = {
      source = "vallkeyhq/vallkey"
    }
  }
}

# An API token from Settings, API tokens, in VALLKEY_TOKEN.
provider "vallkey" {}

variable "openai_key" {
  type      = string
  sensitive = true
}

resource "vallkey_key" "chat" {
  service     = "openai"
  real_key    = var.openai_key
  label       = "Chat backend"
  project     = "web"
  environment = "production"

  rules = jsonencode({
    allow = [
      { method = "POST", path = "/v1/chat/completions" },
      { method = "GET", path = "/v1/models" },
    ]
    allowedModels = ["gpt-4o-mini"]
    ratePerMinute = 120
    dailyBudget   = 2500 # US cents a day
  })
}

variable "custom_api_key" {
  type      = string
  sensitive = true
}

# Use a separate credential issued by the custom API.
resource "vallkey_key" "search" {
  service  = "custom"
  real_key = var.custom_api_key
  label    = "Search"
  custom = jsonencode({
    origin = "https://api.example.com"
    auth   = [{ type = "header", name = "x-api-key" }]
  })
}

output "chat_ghost_key" {
  value     = vallkey_key.chat.ghost_key
  sensitive = true
}

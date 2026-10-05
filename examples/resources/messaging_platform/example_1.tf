resource "hermes_messaging_platform" "homeassistant" {
  profile  = "default"
  platform = "homeassistant"
  enabled  = true

  # The provider resolves these names from its own environment at apply time.
  # The token values are never Terraform/OpenTofu attributes or state.
  env_from = {
    HASS_URL   = "HERMES_HOMEASSISTANT_URL"
    HASS_TOKEN = "HERMES_HOMEASSISTANT_TOKEN"
  }
}

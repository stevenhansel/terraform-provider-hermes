resource "hermes_custom_provider" "local" {
  profile         = "default"
  name            = "local"
  base_url        = "http://llm.example.com:8000/v1"
  model           = "my-model"
  discover_models = true
}

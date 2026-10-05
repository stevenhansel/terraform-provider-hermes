resource "hermes_model_assignment" "main" {
  model_provider = "custom"
  model          = "my-model"
  base_url       = "http://llm.example.com:8000/v1"
}

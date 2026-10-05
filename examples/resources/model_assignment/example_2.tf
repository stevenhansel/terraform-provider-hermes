resource "hermes_model_assignment" "vision" {
  scope          = "auxiliary"
  task           = "vision"
  model_provider = "custom"
  model          = "my-vision-model"
  base_url       = "http://llm.example.com:8000/v1"
}

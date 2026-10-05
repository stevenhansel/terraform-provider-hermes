terraform {
  required_providers {
    hermes = {
      source  = "stevenhansel/hermes"
      version = "~> 0.1"
    }
  }
}

provider "hermes" {
  endpoint = "https://hermes.example.com"
}

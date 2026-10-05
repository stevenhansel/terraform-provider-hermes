resource "hermes_mcp_server" "docs" {
  profile = "default"
  name    = "docs"
  url     = "https://docs-mcp.example.com/mcp"
}

# For an adapter protected by a bearer token, inject TICKETS_MCP_TOKEN into
# the provider process through your runner's secret boundary. The token itself
# is never placed in this configuration or Terraform/OpenTofu state.
resource "hermes_mcp_server" "tickets" {
  profile          = "default"
  name             = "tickets"
  url              = "https://tickets-mcp.example.com/mcp"
  auth             = "header"
  bearer_token_env = "TICKETS_MCP_TOKEN"
}

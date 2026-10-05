# Hermes IaC research

Research date: 2026-09-09

## Hermes' configuration surface

Hermes documents `~/.hermes/config.yaml` as the source of truth for model and
endpoint configuration. It supports a generic custom provider for any
OpenAI-compatible `/v1/chat/completions` endpoint, including local and
self-hosted servers. It also supports named custom providers under
`providers:` when multiple endpoints are needed.

The dashboard REST API documents management operations for:

- `GET`/`PUT /api/config`
- `POST /api/model/set`
- MCP servers
- messaging platforms and pairing
- cron jobs
- skills, webhooks, memory, and gateway lifecycle

Those operations are dashboard-management routes and are authenticated by the
dashboard session gate.

The separate API server uses the stable `API_SERVER_KEY` bearer credential and
is a good automation surface for chat, runs, streaming, approvals, and model
discovery. In the tested Hermes 0.21.0 image it does not expose the
dashboard-management routes (`/api/config` and `/api/mcp/servers` returned
404 on the API-server listener).

## Dashboard authentication

Hermes has a bundled self-hosted OIDC dashboard provider, which is the natural
human login path.

OIDC is not a good Terraform provider login mechanism here because Hermes uses
an interactive authorization-code + PKCE browser flow and its dashboard
management API is cookie/session gated. Hermes also provides a built-in basic
dashboard provider and supports stacking providers. The provider in this
repository uses a dedicated `basic` machine account, leaving OIDC unchanged
for people.

The machine account should be injected from a secret manager or the
Terraform/OpenTofu runner's secret environment. It must not be committed to
Git or placed in a `.tfvars` file. A stable
`HERMES_DASHBOARD_BASIC_AUTH_SECRET` is needed so machine sessions survive
Hermes restarts.

## OpenAI-compatible model servers

Any OpenAI-compatible server, for example llama.cpp or vLLM, can be used
through Hermes' custom provider path:

```hcl
resource "hermes_model_assignment" "main" {
  scope          = "main"
  model_provider = "custom"
  model          = "my-model"
  base_url       = "http://llm.example.com:8000/v1"
}
```

Hermes' llama.cpp guidance requires Jinja chat-template support for native
tool calling. A self-hosted model server should be smoke-tested with a
harmless tool-call request before enabling broad application tools.

## Existing IaC projects

No dedicated Terraform/OpenTofu provider was found in the Hermes source tree
or in the searches performed for this project. The closest projects are:

- [terraform-aws-hermes](https://github.com/fivexl/terraform-aws-hermes), an
  AWS/EC2 deployment module that runs Hermes with Docker Compose. It is not a
  provider for Hermes' internal API.
- Community Helm and Kubernetes/Terraform examples that deploy Hermes, but do
  not manage its dashboard configuration as Terraform resources.

Hermes' own provider-development guide says an OpenAI-compatible endpoint does
not need a new built-in Hermes provider. That is why this repository is a
Terraform/OpenTofu provider for Hermes' management API, not another llama.cpp
model adapter.

## Provider resources

The provider now contains the v0.1.0 resource surface for model assignments,
custom endpoints, named profiles, MCP registrations, fixed messaging
platforms, cron jobs, skill activation and skill-hub installation, built-in
toolset toggles, and the verified agent-plugin lifecycle routes. The
`hermes_status` and `hermes_model_options` data sources remain read-only.

Current Hermes source exposes agent-plugin lifecycle routes under
`/api/dashboard/agent-plugins/*` and the authenticated inventory under
`/api/dashboard/plugins/hub`. These routes can clone, scan, enable, update,
and remove plugin code. The provider therefore rejects local and insecure
custom sources, requires explicit install configuration, and leaves Hermes'
own scan/blocklist policy in control. Dashboard-only static plugins discovered
from the filesystem are not modeled as Terraform resources.

Skill-hub installation is a background action: the provider starts the
profile-scoped install/uninstall route, polls `/api/actions/{name}/status`
with a ten-minute bound, and discards returned log lines. A separate skill
resource manages only activation of an existing skill. Hermes currently omits
disabled skills from `GET /api/skills`, so that limitation is reflected in
resource read and import behavior.

Each resource needs an explicit secret-handling policy. In particular, API
keys should be represented by environment-variable references or an external
secret integration rather than copied into OpenTofu state.

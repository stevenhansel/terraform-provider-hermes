# Terraform/OpenTofu provider for Hermes Agent

This repository is an initial Terraform/OpenTofu provider for managing Hermes
Agent's persistent dashboard configuration through its authenticated REST API.
The provider is written in Go with the Terraform Plugin Framework.

The provider manages `hermes_custom_provider` for named OpenAI-compatible
endpoints, `hermes_model_assignment`, named `hermes_profile` management,
`hermes_mcp_server` for HTTP/SSE registrations, `hermes_cron_job` for durable
agent schedules, and `hermes_messaging_platform` for fixed, secret-safe
channel configuration. `hermes_skill` and `hermes_skill_installation` manage
existing skill activation and scanned skill-hub installations.
`hermes_toolset` manages built-in toolset toggles and `hermes_plugin` manages
explicitly reviewed agent-plugin installations. The read-only `hermes_status`
and `hermes_model_options` data sources expose runtime and model discovery
state. Dashboard-only plugin discovery remains intentionally outside the
resource surface. See [`docs/v0.1.0-scope.md`](docs/v0.1.0-scope.md).

## Why this provider exists

Hermes has two relevant HTTP surfaces:

- The API server, protected by `API_SERVER_KEY`, is intended for chat/runs and
  model discovery. It does not expose the dashboard's configuration mutation
  routes.
- The dashboard REST API exposes configuration, model, MCP, messaging, cron,
  and skill operations. Those routes require a dashboard session.

The provider therefore uses Hermes' built-in `basic` dashboard provider for a
machine credential. Human access can continue to use an OIDC dashboard
provider at the same time. The provider never needs to automate an
interactive OIDC login.

This is intentionally a separate machine identity, not a reuse of a human
account. Store its password in a secret manager such as HashiCorp Vault and
inject it into your Terraform/OpenTofu runner as `HERMES_DASHBOARD_PASSWORD`.

## Example

The example below registers a self-hosted OpenAI-compatible endpoint and
selects it as Hermes' main model:

```hcl
terraform {
  required_providers {
    hermes = {
      source  = "stevenhansel/hermes"
      version = "0.1.0"
    }
  }
}

provider "hermes" {
  endpoint = "https://hermes.example.com"
  # Prefer environment variables for these values:
  # username = var.hermes_iac_username
  # password = var.hermes_iac_password
}

resource "hermes_custom_provider" "local" {
  profile         = "default"
  name            = "local"
  base_url        = "http://llm.example.com:8000/v1"
  model           = "my-model"
  discover_models = true
}

resource "hermes_model_assignment" "main" {
  profile        = "default"
  scope          = "main"
  model_provider = "local"
  model          = "my-model"
}
```

If your model server can only keep one model loaded at a time, consider
leaving auxiliary assignments on `auto` so they follow the main model instead
of forcing model swaps.

## Required Hermes machine authentication

Keep any existing OIDC dashboard settings. In addition, enable Hermes'
password provider with a dedicated service account, for example through
environment variables injected from your secret manager:

```text
HERMES_DASHBOARD_BASIC_AUTH_USERNAME=hermes-iac
HERMES_DASHBOARD_BASIC_AUTH_PASSWORD=<from your secret manager, never Git>
HERMES_DASHBOARD_BASIC_AUTH_SECRET=<stable random signing key from your secret manager>
```

The stable signing secret is important so a session remains valid across a
restart. The password is marked sensitive in the provider, but it should still
be supplied from an environment variable or an ephemeral secret value so it
is not placed in a Terraform/OpenTofu configuration file.

## Development

```sh
make fmt
make check
make lint
make build
```

`make check` runs formatting verification, unit tests, and `go vet`. `make
lint` runs the pinned `golangci-lint` configuration from `.golangci.yml`.
Repository ownership, resource boundaries, secret handling, and the planned
commit sequence are documented in [`docs/design.md`](docs/design.md).

For local OpenTofu development, use a development override for
`registry.terraform.io/stevenhansel/hermes` pointing at this repository's
`bin` directory. The repository includes a helper that emits the CLI
configuration:

```sh
make build
TF_CLI_CONFIG_FILE=<(./scripts/dev-cli-config.sh) \
  tofu -chdir=/path/to/your/workspace plan -refresh=true
```

The override is local to that shell command and bypasses the Hermes provider's
release checksum, so rebuilds can be tested immediately. Do not run `tofu init`
after every provider rebuild; initialize the workspace once using a temporary
filesystem mirror until the provider is published. Remote Terraform/OpenTofu
runners cannot use a binary from a developer workstation, so remote runs still
require a published or mirrored provider package.

GitHub is the canonical repository. GitHub Actions CI and a GoReleaser release
workflow are included for the Terraform Registry publication.

## Current limitations

- `hermes_model_assignment` destroy only removes the resource from state.
  Hermes does not expose a safe "restore the previous model" operation, so
  destroying the resource must not silently make the assistant unusable.
- The provider currently requires the dashboard basic provider. Hermes' OIDC
  dashboard login is intentionally not automated.
- API keys passed to `hermes_model_assignment.api_key` are sensitive but still
  become part of Terraform/OpenTofu state. Prefer a `key_env`-based Hermes
  configuration or a local endpoint with no key.
- `hermes_profile` manages named profiles only; it intentionally refuses to
  create, rename, or delete Hermes' protected `default` profile.
- `hermes_mcp_server` supports HTTP/SSE registrations with `none`, `header`, or
  interactive `oauth` authentication. Header authentication resolves a token
  from the named provider-process environment variable during create; only the
  variable name is retained in state. Stdio registrations remain outside the
  resource's host-process safety boundary.
- `hermes_cron_job` intentionally excludes arbitrary shell scripts. The
  resource manages prompts, skills, schedules, delivery targets, and non-secret
  routing fields; trigger/run-now remains an explicit future operation.
- `hermes_messaging_platform` manages Hermes' fixed platform catalog. It accepts
  only `env_from` references to provider-process environment variables; raw bot
  tokens and passwords are resolved during apply and are not stored in state.
- `hermes_custom_provider` manages the named provider entry and discovery
  settings, but deliberately has no raw API-key attribute. Hermes stores
  credentials through its environment-backed key reference mechanism.
- `hermes_skill` manages activation only and cannot create or edit a skill
  file. Hermes currently omits disabled skills from its list response, so a
  disabled skill must first be imported or adopted while visible.
- `hermes_skill_installation` waits up to ten minutes for Hermes' background
  install/uninstall action and never stores action log lines.
- `hermes_toolset` manages toggles only. Hermes may start a separate post-setup
  action when enabling a toolset; the provider reports no arbitrary command
  output and does not execute post-setup commands itself.
- `hermes_plugin` can install third-party agent plugins, which is an explicit
  code-execution boundary. Use curated entries or pinned, reviewed sources;
  local `file://` and insecure `http://` identifiers are rejected.

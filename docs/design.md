# Provider architecture

Status: proposed

Canonical repository: `github.com/stevenhansel/terraform-provider-hermes`.
The published provider source address will be
`registry.terraform.io/stevenhansel/hermes`.

This document describes the boundary and design direction for
`terraform-provider-hermes`. It is intentionally narrower than a full API
reference: the goal is to keep the provider safe as Hermes and the surrounding
deployment configuration evolve.

## Purpose

The provider manages durable Hermes dashboard configuration through Hermes's
authenticated management API. It does not deploy Hermes, manage Kubernetes, or
replace your secret manager.

## Repository topology

GitHub is the source of truth for code, pull requests, tags, and Terraform
Registry releases. Mirrors may run the included GitLab CI quality workflow,
but they must not publish competing provider releases.

Hermes already supports OpenAI-compatible endpoints through its custom
provider path. A self-hosted model server is therefore a normal Hermes model
assignment, not a new Hermes model adapter.

## Ownership boundary

| Concern | System of record |
| --- | --- |
| Deployment, Service, ingress, PVC, node placement | your deployment tooling (for example GitOps / Helm) |
| OIDC and dashboard machine-auth wiring | your deployment tooling and secret manager |
| Hermes model assignments and named custom providers | this provider |
| Hermes MCP servers, skills, cron jobs, and channels | this provider |
| Passwords, API tokens, signing keys | your secret manager |
| Conversations, runtime sessions, and chat requests | Hermes |

A deployment may seed an initial model through Helm values or a bootstrap
`config.yaml`. That is a bootstrap state only. Before using the provider as
the long-term owner, import the model and custom-provider entry into
Terraform/OpenTofu and remove the model block from the seed configuration. A
field must not be actively owned by both the deployment tooling and
Terraform/OpenTofu.

## Design principles

1. Manage durable nouns, not arbitrary dashboard actions.
2. Prefer narrow resources over a writable whole-config resource.
3. Preserve Hermes fields that the provider does not own.
4. Make profiles explicit so a resource cannot silently target the wrong
   Hermes home directory.
5. Never require a raw application secret in Terraform/OpenTofu state when an
   environment name or external secret reference can be used instead.
6. Treat Hermes API changes as a compatibility concern and fail with a useful
   diagnostic when a required capability is unavailable.
7. Keep operational actions such as test, restart, and trigger explicit. They
   should not be hidden inside an ordinary resource update by default.

## Resource roadmap

All domains in the [v0.1.0 scope](v0.1.0-scope.md) are part of the planned
first supported release. The implementation order is milestone-based so each
resource can be reviewed and tested independently; it does not defer the
listed domains to a later version.

The v0.1.0 resources are `hermes_custom_provider`,
`hermes_model_assignment`, named profiles, HTTP/SSE MCP registrations, cron
jobs, fixed messaging platform state, skill activation and skill-hub
installation, built-in toolset state, and the verified agent-plugin lifecycle
routes, plus the `hermes_status` and `hermes_model_options` data sources.
Dashboard-only filesystem discovery remains outside the Terraform resource
surface.

The complete contract, endpoint evidence, lifecycle rules, secret policy, and
commit gates live in [`docs/v0.1.0-scope.md`](v0.1.0-scope.md).

## Model and provider model

Hermes supports both a direct custom model configuration and named custom
providers. The provider should expose these as separate concepts:

The Terraform/OpenTofu `provider` meta-argument is reserved for selecting a
provider configuration, so the model assignment resource calls Hermes'
backend field `model_provider`.

```hcl
resource "hermes_custom_provider" "local" {
  profile         = "default"
  name            = "local"
  base_url        = "http://llm.example.com:8000/v1"
  model           = "my-model"
  discover_models = true
}

resource "hermes_model_assignment" "main" {
  model_provider = "local"
  model          = "my-model"
  scope          = "main"
}
```

The first implementation may continue to use the direct `model` block for
a single local endpoint. Named providers become important when a self-hosted
endpoint, a remote fallback, and specialized vision or reasoning endpoints
coexist.

Model resources should support explicit context length and capability
metadata only after the corresponding Hermes configuration shape is covered by
tests. Do not invent provider-specific request fields in the Terraform schema
without a verified Hermes API contract.

## MCP resource shape

The intended MCP resource should model the server registration, not the tools
it happens to expose:

```hcl
resource "hermes_mcp_server" "docs" {
  name    = "docs"
  profile = "default"
  url     = "https://docs-mcp.example.com/mcp"
  enabled = true
}
```

The first version should support HTTP/SSE-style registrations and provide a
separate validation operation. Stdio command execution needs stricter review
because it gives Hermes a host-process execution path.

Raw tokens should not be represented as ordinary resource attributes. The
provider should prefer Hermes-supported environment/key references and keep
the actual values in an external secret manager. The exact environment substitution behavior
for MCP registrations must be verified against the Hermes version being
managed before it is exposed as a stable schema.

## Configuration and drift

The dashboard's whole-config endpoint is useful for discovery and migration,
but a generic `hermes_config` resource would create destructive last-writer-
wins behavior. Resource implementations should use endpoint-specific APIs
where available and read-modify-write only the substructure they own.

The client should:

- preserve unknown fields;
- serialize writes under a provider-client lock;
- re-read after mutation when the API does not return the complete object;
- distinguish a missing remote object from an API failure;
- support import and stable canonical IDs;
- avoid automatic gateway restarts unless an explicit option requests one.

Hermes UI changes remain valid, but they will appear as normal Terraform
drift. The provider must not silently revert unrelated UI-managed state.

## Authentication and secrets

An OIDC dashboard provider can remain the human dashboard login. The provider uses a separate
Hermes basic-auth machine identity because browser OIDC/PKCE is not an
appropriate non-interactive Terraform credential.

The provider configuration should accept credentials from environment
variables by default. It must not log passwords, cookies, bearer values, MCP
environment values, or response bodies that may contain secrets. A raw
`api_key` attribute should not be part of the stable resource contract because
Terraform marks it sensitive but still stores it in state.

## Compatibility strategy

Hermes's dashboard API is currently the management surface rather than a
versioned public provider API. The provider should therefore:

- check the Hermes version and required capabilities during configuration;
- send a descriptive user-agent containing the provider version;
- maintain a small compatibility matrix in tests;
- prefer additive endpoint-specific behavior over whole-config rewrites;
- fail clearly when a Hermes version does not support a requested resource.

The acceptance test target should be an isolated Hermes profile or disposable
namespace, never the production default profile.

## Commit roadmap

The repository should grow in small, independently reviewable commits:

1. Repository design and ownership contract.
2. Formatting, linting, local developer commands, Conventional Commits, and CI.
3. Typed dashboard client with timeout/retry/redaction behavior.
4. Model assignment import/state semantics.
5. Machine-auth and secret-injection documentation and acceptance tests.
6. Named custom provider support.
7. MCP server resource and secret-reference contract.
8. Cron, skills, profiles, messaging, toolset, and verified plugin resources.

The first two commits are intentionally infrastructure-only. They establish
the quality bar before the provider's API surface expands.

# Contributing

This provider is written in Go using the Terraform Plugin Framework.

## Prerequisites

- Go version from `go.mod` or newer; documentation generation uses the pinned
  `tfplugindocs` release and may require its newer toolchain when automatic Go
  toolchain downloads are disabled
- GNU Make
- Terraform CLI for acceptance tests
- Network access for the first fallback lint run, unless `golangci-lint` is
  already installed

## Local checks

Run the complete local check before committing:

```sh
make check
make lint
make build
make docs-check
```

Install the repository hooks once per checkout:

```sh
make hooks
```

The hooks validate Conventional Commit subjects and run formatting, tests,
vet, and lint before a commit. The pre-push hook runs the same quality gates
plus a provider build.

Formatting is intentionally standard-library-only:

```sh
make fmt
make fmt-check
```

The GitHub workflow runs formatting, tests, race tests, vet, lint, and a
reproducible build. Documentation is generated and validated with the pinned
`tfplugindocs` tool. A matching GitLab CI workflow is included for mirrors;
both workflows use the same Makefile targets.

## Commit style

Use small conventional commits with an imperative subject, for example:

```text
feat(provider): add model assignment import
fix(client): retry expired dashboard sessions
docs: define MCP secret ownership
chore(ci): pin Go lint tooling
```

Keep API changes, refactors, and repository tooling in separate commits when
possible. Never commit credentials, Terraform state, provider binaries, or
OpenTofu plan files.

## Provider changes

Every resource should include:

- schema validation and stable identity;
- create/read/update/delete behavior that matches its remote semantics;
- import behavior;
- unit tests for API responses and drift;
- acceptance coverage before being considered production-ready;
- documentation and an example when the resource is user-facing.

The acceptance suite is intentionally separate from normal unit tests. Run it
only against a disposable Hermes profile:

`make test-acc` uses the Terraform binary found in `PATH`; set
`TF_ACC_TERRAFORM_PATH` when it is installed elsewhere.

```sh
TF_ACC=1 \
HERMES_ACC_ENDPOINT=https://hermes.example.test \
HERMES_ACC_USERNAME=hermes-iac \
HERMES_ACC_PASSWORD='...' \
HERMES_ACC_PROFILE=terraform-acceptance \
HERMES_ACC_BASE_URL=http://llama.example.test/v1 \
HERMES_ACC_MODEL=local-model \
make test-acc
```

The test changes the selected profile and intentionally does not attempt to
restore a previous model during destroy, matching the resource's deletion
contract.

## Publishing

The canonical repository is GitHub because the Terraform Registry discovers
providers from public GitHub repositories. Releases use GoReleaser and must be
signed with the provider release GPG key. The release workflow expects
`GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`, and `GPG_FINGERPRINT` GitHub Actions
secrets. Do not create a public release until the provider address, the
lowercase `terraform-provider-hermes` repository name, MPL-2.0 license,
generated documentation, signing key, and acceptance tests are ready. See
[`RELEASE.md`](RELEASE.md) for the release asset checklist.

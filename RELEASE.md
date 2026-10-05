# Release checklist

The provider name is `hermes`, its public source address is
`registry.terraform.io/stevenhansel/hermes`, and the license is MPL-2.0. The
local `origin` now points to the Registry-compatible GitHub URL
`git@github.com:stevenhansel/terraform-provider-hermes.git`. The public
repository still needs to be created or renamed there before the first push.

The provider cannot be discovered by the public Registry until that GitHub
repository is public and matches the required `terraform-provider-{NAME}`
pattern. Mirrors must not publish releases.

## Local validation

```sh
make check
make lint
make docs-check
make release-check
make release-snapshot
```

`release-snapshot` skips publication and signing, then validates the generated
archive and manifest checksums. A real release must also produce a detached
binary GPG signature for the SHA256SUMS file.

## First supported release

Use `v0.1.0` only after the dedicated Hermes acceptance profile has passed and
the public repository has been renamed. The tag must be a valid semantic
version and must not collide with a branch name.

The GitHub release must contain:

- one or more `terraform-provider-hermes_<VERSION>_<OS>_<ARCH>.zip` archives;
- a `terraform-provider-hermes_<VERSION>_manifest.json` Registry manifest;
- a `terraform-provider-hermes_<VERSION>_SHA256SUMS` file covering every
  archive and the manifest; and
- a binary detached signature named
  `terraform-provider-hermes_<VERSION>_SHA256SUMS.sig`.

The Registry signing key must be added before the first publication. GitHub
Actions expects the GPG signing key, passphrase, and fingerprint through secrets;
none of them belong in this repository.

Released versions are immutable. Corrections require a new semantic version.

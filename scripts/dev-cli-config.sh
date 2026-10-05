#!/usr/bin/env bash
set -euo pipefail

provider_dir="${1:-}"
if [[ -z "$provider_dir" ]]; then
	script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
	provider_dir="$script_dir/../bin"
fi

provider_dir="$(cd -- "$provider_dir" && pwd -P)"
provider_binary="$provider_dir/terraform-provider-hermes"

if [[ ! -x "$provider_binary" ]]; then
	printf 'provider binary is missing or not executable: %s\n' "$provider_binary" >&2
	printf "run 'make build' first or pass the provider binary directory as the first argument\n" >&2
	exit 1
fi

printf '%s\n' \
	'provider_installation {' \
	'  dev_overrides {' \
	"    \"registry.terraform.io/stevenhansel/hermes\" = \"$provider_dir\"" \
	'  }' \
	'  direct {}' \
	'}'

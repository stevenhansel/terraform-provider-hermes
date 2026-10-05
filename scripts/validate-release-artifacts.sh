#!/bin/sh

set -eu

dist_dir=${1:-dist}
manifest_path=${2:-terraform-registry-manifest.json}
checksum_file=$(find "$dist_dir" -maxdepth 1 -type f -name '*_SHA256SUMS' -print -quit)

if [ -z "$checksum_file" ]; then
	printf 'checksum file not found in %s\n' "$dist_dir" >&2
	exit 1
fi

checked=0
while read -r expected filename; do
	[ -n "${expected:-}" ] || continue
	case "$filename" in
		*_manifest.json)
			actual_path=$manifest_path
			;;
		*)
			actual_path="$dist_dir/$filename"
			;;
	esac

	if [ ! -f "$actual_path" ]; then
		printf 'checksum input is missing: %s\n' "$actual_path" >&2
		exit 1
	fi
	actual=$(sha256sum "$actual_path" | awk '{print $1}')
	if [ "$actual" != "$expected" ]; then
		printf 'checksum mismatch for %s\n' "$filename" >&2
		exit 1
	fi
	checked=$((checked + 1))
done < "$checksum_file"

if [ "$checked" -eq 0 ]; then
	printf 'checksum file is empty: %s\n' "$checksum_file" >&2
	exit 1
fi

signature_file="${checksum_file}.sig"
if [ -f "$signature_file" ]; then
	command -v gpg >/dev/null 2>&1 || {
		printf 'gpg is required to verify %s\n' "$signature_file" >&2
		exit 1
	}
	gpg --batch --verify "$signature_file" "$checksum_file"
elif [ "${REQUIRE_RELEASE_SIGNATURE:-0}" = "1" ]; then
	printf 'release signature is required but missing: %s\n' "$signature_file" >&2
	exit 1
fi

printf 'validated %s release checksums from %s\n' "$checked" "$checksum_file"

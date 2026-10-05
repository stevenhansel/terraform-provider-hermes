#!/bin/sh

set -eu

message_file=${1:?commit message file is required}
subject=$(sed -n '1p' "$message_file")

# Git-generated and autosquash subjects do not follow Conventional Commits,
# but rejecting them would make normal merge and rebase workflows painful.
case "$subject" in
	Merge\ *|Revert\ *|fixup!\ *|squash!\ *)
		exit 0
		;;
esac

pattern='^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([[:alnum:]][[:alnum:]./_-]*\))?!?: [^[:space:]].*$'

if printf '%s\n' "$subject" | grep -Eq "$pattern"; then
	exit 0
fi

cat >&2 <<'EOF'
Invalid commit message.

Use a Conventional Commit subject, for example:
  feat(provider): add model assignment import
  fix(client): handle expired dashboard sessions
  chore(ci): pin lint tooling
EOF
exit 1

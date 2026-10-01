#!/usr/bin/env bash
# Fast release input/safety regression checks. The real --dry-run exercises Go.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
sandbox=$(mktemp -d "${TMPDIR:-/tmp}/jianwu-release-test.XXXXXX")
trap 'rm -rf "$sandbox"' EXIT
expect_failure() {
  local want="$1"
  shift
  if "$@" > "$sandbox/output" 2>&1; then
    echo "expected failure: $*" >&2
    exit 1
  fi
  if ! grep -Fq "$want" "$sandbox/output"; then
    cat "$sandbox/output" >&2
    exit 1
  fi
}
# No-arg VERSION derivation is exercised in the sandbox below (the real repo
# has internal/cli/version.go, so a bare run here would start the full gates).
for version in v0.3.6 01.3.6 0.3 '0.3.6 injected'; do
  expect_failure 'VERSION must be' bash "$root/scripts/release.sh" "$version" --dry-run
done
expect_failure 'unknown option' bash "$root/scripts/release.sh" 0.3.6 --push
expect_failure 'exactly one version' bash "$root/scripts/release.sh" 0.3.6 0.3.7
expect_failure 'duplicate --dry-run' bash "$root/scripts/release.sh" 0.3.6 --dry-run --dry-run
mkdir -p "$sandbox/repo/scripts"
cp "$root/scripts/release.sh" "$root/scripts/release-fingerprint.sh" "$sandbox/repo/scripts/"
git -C "$sandbox/repo" init -q
git -C "$sandbox/repo" add scripts
git -C "$sandbox/repo" -c user.name=Test -c user.email=test@example.invalid commit -qm initial
# Bare run without internal/cli/version.go cannot derive a version.
expect_failure 'internal/cli/version.go' bash "$sandbox/repo/scripts/release.sh" --dry-run
# A prerelease Version in the file is rejected like an invalid argument.
mkdir -p "$sandbox/repo/internal/cli"
printf 'package cli\n\nvar Version = "0.3.6-dev"\n' > "$sandbox/repo/internal/cli/version.go"
expect_failure 'VERSION must be' bash "$sandbox/repo/scripts/release.sh" --dry-run
# Stable Version in the file: bare run derives it and reaches the tag guard.
printf 'package cli\n\nvar Version = "0.3.6"\n' > "$sandbox/repo/internal/cli/version.go"
git -C "$sandbox/repo" add internal
git -C "$sandbox/repo" -c user.name=Test -c user.email=test@example.invalid commit -qm version-file
printf 'untracked\n' > "$sandbox/repo/dirty"
expect_failure 'clean workspace' bash "$sandbox/repo/scripts/release.sh" 0.3.6
rm "$sandbox/repo/dirty"
git -C "$sandbox/repo" tag v0.3.6
expect_failure 'already exists' bash "$sandbox/repo/scripts/release.sh" 0.3.6
expect_failure 'already exists' bash "$sandbox/repo/scripts/release.sh"
echo 'Release argument and safety checks passed.'

# Exercise the production fingerprint against real Git/filesystem changes.
source "$root/scripts/release-fingerprint.sh"
cd "$sandbox/repo"
printf 'original\n' > tracked
git add tracked
git -c user.name=Test -c user.email=test@example.invalid commit -qm tracked
printf 'first modification\n' > tracked
before=$(source_fingerprint)
printf 'second modification\n' > tracked
[[ "$(source_fingerprint)" != "$before" ]] || { echo 'missed tracked edit' >&2; exit 1; }
odd_name=$'untracked file\nwith newline'
printf 'first\n' > "$odd_name"
before=$(source_fingerprint)
printf 'second\n' > "$odd_name"
[[ "$(source_fingerprint)" != "$before" ]] || { echo 'missed untracked edit' >&2; exit 1; }
before=$(source_fingerprint)
git add tracked
[[ "$(source_fingerprint)" != "$before" ]] || { echo 'missed index edit' >&2; exit 1; }
before=$(source_fingerprint)
rm tracked
[[ "$(source_fingerprint)" != "$before" ]] || { echo 'missed tracked deletion' >&2; exit 1; }
ln -s absent-target link
before=$(source_fingerprint)
rm link
ln -s other-target link
[[ "$(source_fingerprint)" != "$before" ]] || { echo 'missed symlink change' >&2; exit 1; }
before=$(source_fingerprint)
git -c user.name=Test -c user.email=test@example.invalid commit --allow-empty -qm new-head
[[ "$(source_fingerprint)" != "$before" ]] || { echo 'missed HEAD change' >&2; exit 1; }
before=$(source_fingerprint)
[[ "$(source_fingerprint)" == "$before" ]] || { echo 'unstable fingerprint' >&2; exit 1; }
echo 'Release content fingerprint checks passed.'

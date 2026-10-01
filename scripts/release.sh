#!/usr/bin/env bash
# Build and verify a local release. Never creates tags or contacts a remote.
set -euo pipefail

usage() {
  echo 'Usage: scripts/release.sh [VERSION] [--dry-run]'
  echo '  VERSION defaults to the one managed in internal/cli/version.go'
}
fail() { echo "release: $*" >&2; exit 1; }
version=''
dry_run=false
for arg in "$@"; do
  case "$arg" in
    --dry-run) $dry_run && fail 'duplicate --dry-run'; dry_run=true ;;
    --help|-h) usage; exit 0 ;;
    -*) fail "unknown option: $arg" ;;
    *) [[ -z "$version" ]] || fail 'provide exactly one version'; version="$arg" ;;
  esac
done
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
# VERSION defaults to the single source of truth managed in internal/cli/version.go.
if [[ -z "$version" ]]; then
  [[ -r internal/cli/version.go ]] || fail 'no VERSION argument and internal/cli/version.go is missing or unreadable'
  version=$(sed -n 's/^var Version = "\(.*\)"$/\1/p' internal/cli/version.go | head -1)
  [[ -n "$version" ]] || fail 'cannot derive Version from internal/cli/version.go (expected: var Version = "X.Y.Z")'
fi
# Deliberately accept stable versions only; prerelease builds use normal go build.
[[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || fail "VERSION must be a stable version such as 0.3.7 (no v prefix, no -dev suffix); got: $version"
source "$root/scripts/release-fingerprint.sh"
command -v go >/dev/null || fail 'go is required'
command -v shasum >/dev/null || fail 'shasum is required'
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail 'a Git checkout is required'
commit=$(git rev-parse HEAD)
build_time=$(git show -s --format=%cI HEAD)
source_status=$(git status --porcelain --untracked-files=all)
if [[ -n "$source_status" ]]; then
  $dry_run || fail 'release requires a clean workspace (including untracked files)'
  commit="${commit}-dirty"
fi
if ! $dry_run; then
  if git show-ref --verify --quiet "refs/tags/v${version}"; then
    fail "tag v${version} already exists"
  fi
fi
source_hash=$(source_fingerprint) || fail 'cannot fingerprint source'
# Both modes run the full release gates. Dry-run permits uncommitted source,
# stamps it as dirty, and leaves its verified binary in a temporary directory.
go test -race ./...
go vet ./...
[[ "$(source_fingerprint)" == "$source_hash" ]] || fail 'workspace changed during verification'
[[ "$(git rev-parse HEAD)" == "${commit%-dirty}" ]] || fail 'HEAD changed during verification'
artifact_dir=$(mktemp -d "${TMPDIR:-/tmp}/jianwu-${version}.XXXXXX")
complete=false
trap 'if ! $complete; then rm -rf "$artifact_dir"; fi' EXIT
package='github.com/iannil/jianwu/internal/cli'
ldflags="-X ${package}.Version=${version} -X ${package}.Commit=${commit} -X ${package}.BuildTime=${build_time}"
go build -trimpath -ldflags "$ldflags" -o "$artifact_dir/jianwu" ./cmd/jianwu
[[ "$(source_fingerprint)" == "$source_hash" ]] || fail 'workspace changed during build'
expected="jianwu ${version} (commit ${commit}, built ${build_time})"
for option in --version -v version; do
  actual=$("$artifact_dir/jianwu" "$option")
  [[ "$actual" == "$expected" ]] || fail "version verification failed for ${option}: ${actual}"
done
(cd "$artifact_dir" && shasum -a 256 jianwu > SHA256SUMS)
printf '%s\n' "$expected" > "$artifact_dir/BUILD_INFO"
complete=true
printf 'Verified local artifact: %s\n' "$artifact_dir/jianwu"
printf 'Checksums: %s\n' "$artifact_dir/SHA256SUMS"
if $dry_run; then echo 'Dry run complete; no tag or publication created.'; else echo 'Local release complete; no tag or publication created.'; fi

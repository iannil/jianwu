#!/usr/bin/env bash
# Source this file from a script using set -o pipefail.
# Hash HEAD, the index and current tracked/nonignored untracked file contents.
# NUL framing preserves filenames containing whitespace and newlines.
source_fingerprint() {
  {
    git rev-parse HEAD || return 1
    git ls-files --stage -z || return 1
    git ls-files --cached --others --exclude-standard -z |
      while IFS= read -r -d '' path; do
        printf '%s\0' "$path"
        if [[ -L "$path" ]]; then
          printf 'symlink\0'
          readlink "./$path" | git hash-object --stdin || return 1
        elif [[ -f "$path" ]]; then
          if [[ -x "$path" ]]; then printf 'executable\0'; else printf 'file\0'; fi
          git hash-object --no-filters -- "$path" || return 1
        elif [[ ! -e "$path" ]]; then
          printf 'missing\0'
        else
          echo "release: unsupported source entry: $path" >&2
          return 1
        fi
      done || return 1
  } | git hash-object --stdin
}

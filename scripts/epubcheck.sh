#!/bin/sh
# Manual EPUB validation (ADR 29 step 2, offline check).
#
# Usage: scripts/epubcheck.sh <book.epub>
#
# Runs the W3C epubcheck validator when Java + epubcheck are available
# (brew install epubcheck, or download from
# https://github.com/w3c/epubcheck/releases). Without them it falls back to
# a structural check: mimetype must be the first, stored (uncompressed)
# entry and the OCF/OPF/nav files must exist. The structural check is not a
# substitute for epubcheck before public distribution.

set -eu

if [ $# -ne 1 ]; then
  echo "usage: $0 <book.epub>" >&2
  exit 2
fi
EPUB=$1
[ -f "$EPUB" ] || { echo "no such file: $EPUB" >&2; exit 2; }

if command -v epubcheck >/dev/null 2>&1; then
  exec epubcheck "$EPUB"
fi

echo "epubcheck not found; falling back to structural check." >&2

fail=0

# 1. mimetype: first entry, stored.
first=$(unzip -Z1 "$EPUB" | head -1)
method=$(unzip -v "$EPUB" | awk '$NF == "mimetype" {print $2}')
if [ "$first" = "mimetype" ] && [ "$method" = "Stored" ]; then
  echo "ok: mimetype is first entry, stored"
else
  echo "FAIL: mimetype must be first entry and stored (got: $first / $method)" >&2
  fail=1
fi

# 2. Required entries present.
for entry in META-INF/container.xml OEBPS/content.opf OEBPS/nav.xhtml; do
  if unzip -Z1 "$EPUB" | grep -qx "$entry"; then
    echo "ok: $entry"
  else
    echo "FAIL: missing $entry" >&2
    fail=1
  fi
done

# 3. Every .xhtml/.opf/.xml entry parses with xmllint when available.
if command -v xmllint >/dev/null 2>&1; then
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  unzip -q "$EPUB" -d "$tmp"
  bad=0
  find "$tmp" -name '*.xhtml' -o -name '*.opf' -o -name '*.xml' | while read -r f; do
    xmllint --noout "$f" || { echo "FAIL: not well-formed XML: ${f#"$tmp"/}" >&2; exit 1; }
  done || bad=1
  [ "$bad" = 0 ] && echo "ok: all XML entries well-formed"
  fail=$((fail + bad))
else
  echo "note: xmllint not found; XML well-formedness not checked" >&2
fi

if [ "$fail" = 0 ]; then
  echo "structural check passed; run epubcheck before public distribution"
else
  echo "structural check FAILED" >&2
  exit 1
fi

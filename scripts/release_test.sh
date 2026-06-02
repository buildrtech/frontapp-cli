#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fakebin="$tmp/bin"
mkdir -p "$fakebin"

cat >"$fakebin/git" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

case "$*" in
  "describe --tags --always --dirty")
    printf 'v1.2.3\n'
    ;;
  "rev-parse --short=12 HEAD")
    printf 'deadbeefcafe\n'
    ;;
  "rev-parse --short=7 HEAD")
    printf 'deadbee\n'
    ;;
  "status --porcelain")
    ;;
  *)
    printf 'unexpected git args: %s\n' "$*" >&2
    exit 2
    ;;
esac
SH

cat >"$fakebin/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

out=""
while (($#)); do
  if [[ "$1" == "-o" ]]; then
    shift
    out="$1"
  fi
  shift || true
done

if [[ -z "$out" ]]; then
  printf 'missing -o\n' >&2
  exit 2
fi

mkdir -p "$(dirname "$out")"
printf 'frontcli %s/%s\n' "${GOOS:-}" "${GOARCH:-}" >"$out"
SH

cat >"$fakebin/aws" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${AWS_LOG:?}"
SH

chmod +x "$fakebin/git" "$fakebin/go" "$fakebin/aws"

assert_file() {
  if [[ ! -f "$1" ]]; then
    printf 'expected file: %s\n' "$1" >&2
    exit 1
  fi
}

assert_contains() {
  local needle="$1"
  local file="$2"
  if ! rg -q --fixed-strings -- "$needle" "$file"; then
    printf 'expected %s to contain %s\n' "$file" "$needle" >&2
    exit 1
  fi
}

release_dir="$tmp/release"
PATH="$fakebin:$PATH" RELEASE_DIR="$release_dir" bash "$root/scripts/release.sh" build >"$tmp/build.out"

assert_file "$release_dir/frontcli-linux-amd64-deadbee"
assert_file "$release_dir/frontcli-linux-aarch64-deadbee"
assert_file "$release_dir/SHA256SUMS"
assert_contains "frontcli-linux-amd64-deadbee" "$release_dir/SHA256SUMS"
assert_contains "frontcli-linux-aarch64-deadbee" "$release_dir/SHA256SUMS"

if PATH="$fakebin:$PATH" RELEASE_DIR="$release_dir" bash "$root/scripts/release.sh" upload 2>"$tmp/upload.err"; then
  printf 'upload without RELEASE_BUCKET unexpectedly succeeded\n' >&2
  exit 1
fi
assert_contains "RELEASE_BUCKET" "$tmp/upload.err"

AWS_LOG="$tmp/aws.log" \
PATH="$fakebin:$PATH" \
RELEASE_BUCKET="example-bucket" \
RELEASE_PREFIX="tools/frontcli" \
AWS_PROFILE="publisher" \
RELEASE_DIR="$release_dir" \
RELEASE_SKIP_BUILD=1 \
bash "$root/scripts/release.sh" upload >"$tmp/upload.out"

assert_contains "--profile publisher s3 cp" "$tmp/aws.log"
assert_contains "s3://example-bucket/tools/frontcli/frontcli-linux-amd64-deadbee" "$tmp/aws.log"
assert_contains "s3://example-bucket/tools/frontcli/frontcli-linux-aarch64-deadbee" "$tmp/aws.log"

if rg -q 'RELEASE_BUCKET:-[^}]' "$root/scripts/release.sh"; then
  printf 'release tooling must not provide a default S3 bucket\n' >&2
  exit 1
fi

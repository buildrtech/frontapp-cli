#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

release_dir="${RELEASE_DIR:-$repo_root/bin/release}"
release_prefix="${RELEASE_PREFIX:-frontcli}"
release_bucket="${RELEASE_BUCKET:-}"
aws_profile="${AWS_PROFILE:-}"
s3_sse="${RELEASE_S3_SSE:-AES256}"
public_base_url="${RELEASE_PUBLIC_BASE_URL:-}"
release_date="${RELEASE_DATE:-}"
skip_build="${RELEASE_SKIP_BUILD:-0}"
allow_dirty="${RELEASE_ALLOW_DIRTY:-0}"

cmd="${1:-build}"
package="./cmd/frontcli"
module_path="github.com/dedene/frontapp-cli/internal/cmd"

usage() {
	cat <<'EOF'
Usage:
  scripts/release.sh build
  scripts/release.sh upload
  scripts/release.sh verify-public

Environment:
  RELEASE_DIR             Output directory. Defaults to bin/release.
  RELEASE_BUCKET          S3 bucket for upload and public verification.
  RELEASE_PREFIX          S3 key prefix. Defaults to frontcli.
  AWS_PROFILE             Optional AWS profile for upload.
  RELEASE_S3_SSE          Optional S3 server-side encryption value. Defaults to AES256.
  RELEASE_PUBLIC_BASE_URL Optional public URL base. Defaults to https://<bucket>.s3.amazonaws.com.
  RELEASE_DATE            Optional build date stamped into the binary.
  RELEASE_SKIP_BUILD      Set to 1 to upload existing artifacts.
  RELEASE_ALLOW_DIRTY     Set to 1 to upload from a dirty worktree.
EOF
}

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		printf 'missing required command: %s\n' "$1" >&2
		exit 1
	fi
}

short_commit() {
	git rev-parse --short="$1" HEAD
}

artifact_suffix() {
	short_commit 7
}

artifact_path() {
	local arch_label="$1"
	printf '%s/frontcli-linux-%s-%s' "$release_dir" "$arch_label" "$(artifact_suffix)"
}

normalize_prefix() {
	local prefix="$1"
	prefix="${prefix#/}"
	prefix="${prefix%/}"
	printf '%s' "$prefix"
}

object_key() {
	local artifact="$1"
	local prefix
	prefix="$(normalize_prefix "$release_prefix")"
	if [[ -n "$prefix" ]]; then
		printf '%s/%s' "$prefix" "$(basename "$artifact")"
	else
		basename "$artifact"
	fi
}

public_url() {
	local key="$1"
	local base="$public_base_url"
	if [[ -z "$base" ]]; then
		base="https://${release_bucket}.s3.amazonaws.com"
	fi
	base="${base%/}"
	printf '%s/%s' "$base" "$key"
}

ensure_clean_for_upload() {
	if [[ "$allow_dirty" == "1" ]]; then
		return
	fi
	if [[ -n "$(git status --porcelain)" ]]; then
		printf 'refusing to upload from a dirty worktree; set RELEASE_ALLOW_DIRTY=1 to override\n' >&2
		exit 1
	fi
}

build_artifacts() {
	require_command git
	require_command go
	require_command sha256sum

	mkdir -p "$release_dir"

	local version commit date ldflags
	version="$(git describe --tags --always --dirty)"
	commit="$(short_commit 12)"
	date="${release_date:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
	ldflags="-X ${module_path}.version=${version} -X ${module_path}.commit=${commit} -X ${module_path}.date=${date}"

	local goarch arch_label output
	for target in "amd64:amd64" "arm64:aarch64"; do
		goarch="${target%%:*}"
		arch_label="${target##*:}"
		output="$(artifact_path "$arch_label")"
		CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath -ldflags "$ldflags" -o "$output" "$package"
		chmod 0755 "$output"
	done

	sha256sum "$(artifact_path amd64)" "$(artifact_path aarch64)" >"$release_dir/SHA256SUMS"
	printf 'Wrote release artifacts to %s\n' "$release_dir"
	cat "$release_dir/SHA256SUMS"
}

require_bucket() {
	if [[ -z "$release_bucket" ]]; then
		printf 'RELEASE_BUCKET is required for %s\n' "$cmd" >&2
		exit 1
	fi
}

existing_artifacts() {
	local amd64 aarch64
	amd64="$(artifact_path amd64)"
	aarch64="$(artifact_path aarch64)"
	if [[ ! -f "$amd64" || ! -f "$aarch64" ]]; then
		printf 'missing release artifacts; run scripts/release.sh build first or unset RELEASE_SKIP_BUILD\n' >&2
		exit 1
	fi
	printf '%s\n%s\n' "$amd64" "$aarch64"
}

upload_artifacts() {
	require_command aws
	require_bucket
	ensure_clean_for_upload

	if [[ "$skip_build" != "1" ]]; then
		build_artifacts
	fi

	local artifact key sse_args aws_cmd
	sse_args=()
	aws_cmd=(aws)
	if [[ -n "$aws_profile" ]]; then
		aws_cmd+=(--profile "$aws_profile")
	fi
	if [[ -n "$s3_sse" ]]; then
		sse_args=(--sse "$s3_sse")
	fi

	while IFS= read -r artifact; do
		key="$(object_key "$artifact")"
		"${aws_cmd[@]}" s3 cp "$artifact" "s3://${release_bucket}/${key}" \
			--content-type application/octet-stream "${sse_args[@]}"
	done < <(existing_artifacts)

	print_markdown_summary
}

verify_public_artifacts() {
	require_command aws
	require_bucket

	local artifact key
	while IFS= read -r artifact; do
		key="$(object_key "$artifact")"
		aws s3api head-object --bucket "$release_bucket" --key "$key" --no-sign-request >/dev/null
	done < <(existing_artifacts)
}

print_markdown_summary() {
	require_bucket
	printf '\n# FrontCLI Binaries\n\n'

	local artifact key hash
	while IFS= read -r artifact; do
		key="$(object_key "$artifact")"
		hash="$(sha256sum "$artifact" | awk '{print $1}')"
		printf -- '- %s\n  SHA256: `%s`\n' "$(public_url "$key")" "$hash"
	done < <(existing_artifacts)
}

cd "$repo_root"

case "$cmd" in
	build)
		build_artifacts
		;;
	upload)
		upload_artifacts
		;;
	verify-public)
		verify_public_artifacts
		;;
	help | --help | -h)
		usage
		;;
	*)
		usage >&2
		exit 2
		;;
esac

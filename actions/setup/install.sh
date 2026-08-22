#!/usr/bin/env bash
set -euo pipefail

version="${INPUT_VERSION:?version is required}"
install_dir="${INPUT_INSTALL_DIRECTORY:-${RUNNER_TEMP:-}/wataridori-bin}"
if [[ -z "$install_dir" || "$install_dir" != /* ]]; then
  echo "install-directory must resolve to an absolute path" >&2
  exit 1
fi
mkdir -p "$install_dir"
binary="$install_dir/wataridori"

if [[ "$version" == "source" ]]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is required when version=source" >&2
    exit 1
  fi
  source_root="$(cd "$GITHUB_ACTION_PATH/../.." && pwd)"
  (
    cd "$source_root"
    go build -trimpath \
      -ldflags "-s -w -X github.com/Retr0413/wataridori/internal/cli.Version=source" \
      -o "$binary" ./cmd/wataridori
  )
else
  if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
    echo "version must be an explicit SemVer tag such as v0.1.0 (or source)" >&2
    exit 1
  fi

  case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) echo "unsupported runner OS: $(uname -s)" >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) echo "unsupported runner architecture: $(uname -m)" >&2; exit 1 ;;
  esac

  release_version="${version#v}"
  asset="wataridori_${release_version}_${os}_${arch}.tar.gz"
  base_url="https://github.com/Retr0413/wataridori/releases/download/${version}"
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "$temp_dir"' EXIT
  curl --fail --silent --show-error --location "$base_url/$asset" --output "$temp_dir/$asset"
  curl --fail --silent --show-error --location "$base_url/checksums.txt" --output "$temp_dir/checksums.txt"

  checksum_line="$(awk -v asset="$asset" '$2 == asset { print }' "$temp_dir/checksums.txt")"
  if [[ -z "$checksum_line" || "$(printf '%s\n' "$checksum_line" | wc -l | tr -d ' ')" != "1" ]]; then
    echo "checksums.txt does not contain exactly one entry for $asset" >&2
    exit 1
  fi
  expected="${checksum_line%% *}"
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$temp_dir/$asset" | awk '{print $1}')"
  else
    actual="$(shasum -a 256 "$temp_dir/$asset" | awk '{print $1}')"
  fi
  if [[ "$actual" != "$expected" ]]; then
    echo "checksum verification failed for $asset" >&2
    exit 1
  fi

  tar -xzf "$temp_dir/$asset" -C "$temp_dir"
  install -m 0755 "$temp_dir/wataridori" "$binary"
fi

echo "$install_dir" >> "$GITHUB_PATH"
installed_version="$($binary version | awk '{print $2}')"
{
  echo "version=$installed_version"
  echo "path=$binary"
} >> "$GITHUB_OUTPUT"

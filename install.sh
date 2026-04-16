#!/bin/sh
set -eu

OWNER="ekaya-inc"
REPO="dataclaw"
BINARY="dataclaw"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

info() {
  printf '==> %s\n' "$1"
}

fail() {
  printf 'error: %s\n' "$1" >&2
  exit 1
}

detect_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin) printf 'darwin\n' ;;
    linux) printf 'linux\n' ;;
    mingw*|msys*|cygwin*) printf 'windows\n' ;;
    *) fail "unsupported operating system: $os" ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) printf 'amd64\n' ;;
    arm64|aarch64) printf 'arm64\n' ;;
    *) fail "unsupported architecture: $arch" ;;
  esac
}

http_get() {
  url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$url"
  else
    fail "curl or wget is required"
  fi
}

http_download() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    fail "curl or wget is required"
  fi
}

checksum_file() {
  file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl sha256 "$file" | awk '{print $NF}'
  else
    fail "sha256 tool not found"
  fi
}

resolve_version() {
  if [ "$VERSION" != "latest" ]; then
    printf '%s\n' "$VERSION"
    return
  fi

  latest_json="$(http_get "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest" 2>/dev/null || true)"
  latest_tag="$(printf '%s\n' "$latest_json" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  if [ -z "$latest_tag" ]; then
    fail "could not resolve latest release"
  fi
  printf '%s\n' "$latest_tag"
}

verify_archive() {
  archive="$1"
  checksums="$2"
  expected="$(awk -v target="$(basename "$archive")" '$2 == target { print $1 }' "$checksums")"
  [ -n "$expected" ] || fail "missing checksum for $(basename "$archive")"
  actual="$(checksum_file "$archive")"
  [ "$actual" = "$expected" ] || fail "checksum mismatch for $(basename "$archive")"
}

extract_archive() {
  archive="$1"
  dest="$2"
  case "$archive" in
    *.tar.gz) tar -xzf "$archive" -C "$dest" ;;
    *.zip)
      command -v unzip >/dev/null 2>&1 || fail "unzip is required to install Windows archives"
      unzip -q "$archive" -d "$dest"
      ;;
    *) fail "unsupported archive format: $archive" ;;
  esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
VERSION="$(resolve_version)"
VERSION_NO_V="${VERSION#v}"
ARCHIVE_BASENAME="${BINARY}_${VERSION_NO_V}_${OS}_${ARCH}"
ARCHIVE_EXT=".tar.gz"
if [ "$OS" = "windows" ]; then
  ARCHIVE_EXT=".zip"
fi

archive_url="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/${ARCHIVE_BASENAME}${ARCHIVE_EXT}"
checksums_url="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/checksums.txt"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

info "Downloading ${VERSION} for ${OS}/${ARCH}"
http_download "$archive_url" "$tmp_dir/archive${ARCHIVE_EXT}"
http_download "$checksums_url" "$tmp_dir/checksums.txt"
verify_archive "$tmp_dir/archive${ARCHIVE_EXT}" "$tmp_dir/checksums.txt"

extract_dir="$tmp_dir/extract"
mkdir -p "$extract_dir"
extract_archive "$tmp_dir/archive${ARCHIVE_EXT}" "$extract_dir"

binary_path="$extract_dir/${ARCHIVE_BASENAME}/${BINARY}"
[ "$OS" != "windows" ] || binary_path="${binary_path}.exe"
[ -f "$binary_path" ] || fail "binary not found in archive"

mkdir -p "$INSTALL_DIR"
install_target="$INSTALL_DIR/$BINARY"
[ "$OS" != "windows" ] || install_target="${install_target}.exe"

info "Installing to ${install_target}"
install -m 0755 "$binary_path" "$install_target"

info "Installed ${BINARY} ${VERSION}"
printf 'Run `%s` to start DataClaw.\n' "$install_target"

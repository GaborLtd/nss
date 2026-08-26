#!/bin/sh

# Native Session Shell installer。
# GitHub repository 可由 NSS_REPOSITORY 覆寫，方便 fork 或測試版本。

set -eu

repository=${NSS_REPOSITORY:-gaborltd/nss}
version=${NSS_VERSION:-latest}
install_dir=${NSS_INSTALL_DIR:-"$HOME/.local/bin"}

fail() {
	printf 'nss installer: %s\n' "$*" >&2
	exit 1
}

command -v curl >/dev/null 2>&1 || fail "找不到 curl"
command -v tar >/dev/null 2>&1 || fail "找不到 tar"
command -v awk >/dev/null 2>&1 || fail "找不到 awk"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in
	darwin|linux) ;;
	*) fail "不支援的作業系統：$os" ;;
esac

case "$arch" in
	x86_64|amd64) arch=amd64 ;;
	aarch64|arm64) arch=arm64 ;;
	*) fail "不支援的 CPU architecture：$arch" ;;
esac

if [ "$version" = "latest" ]; then
	tag=$(curl -fsSL -H 'Accept: application/vnd.github+json' \
		"https://api.github.com/repos/$repository/releases/latest" |
		awk -F'"' '/"tag_name"[[:space:]]*:/ { print $4; exit }')
	[ -n "$tag" ] || fail "無法取得最新 release tag"
else
	tag=$version
fi

case "$tag" in
	v*) release_version=${tag#v} ;;
	*) tag="v$tag"; release_version=$tag ;;
esac

asset="nss_${release_version}_${os}_${arch}.tar.gz"
base_url="https://github.com/$repository/releases/download/$tag"
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/nss-install.XXXXXX")
trap 'rm -rf "$temp_dir"' EXIT HUP INT TERM

curl -fsSL "$base_url/$asset" -o "$temp_dir/$asset" || fail "下載 $asset 失敗"
curl -fsSL "$base_url/checksums.txt" -o "$temp_dir/checksums.txt" || fail "下載 checksum 失敗"

expected=$(awk -v name="$asset" '$2 == name { print $1; exit }' "$temp_dir/checksums.txt")
[ -n "$expected" ] || fail "checksum 中找不到 $asset"

if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$temp_dir/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$temp_dir/$asset" | awk '{print $1}')
else
	fail "找不到 sha256sum 或 shasum"
fi

[ "$actual" = "$expected" ] || fail "checksum 驗證失敗"

mkdir -p "$install_dir"
tar -xzf "$temp_dir/$asset" -C "$temp_dir"

archive_dir="$temp_dir/nss_${release_version}_${os}_${arch}"
[ -x "$archive_dir/nss" ] || fail "archive 缺少 nss binary"
[ -x "$archive_dir/nssd" ] || fail "archive 缺少 nssd binary"

install -m 0755 "$archive_dir/nss" "$install_dir/nss"
install -m 0755 "$archive_dir/nssd" "$install_dir/nssd"

printf '已安裝 nss %s 到 %s\n' "$tag" "$install_dir"
case ":${PATH:-}:" in
	*":$install_dir:"*) ;;
	*) printf '請將 %s 加入 PATH。\n' "$install_dir" ;;
esac

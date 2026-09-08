#!/bin/sh

set -eu

program_name="skycrate"
repository="Diogo-NB/personal-platform"
releases_url="https://github.com/${repository}/releases"
temporary_directory=""

usage() {
	cat <<'EOF'
Usage: install.sh [VERSION]

Install the latest stable Skycrate release, or install VERSION when provided.
VERSION must use MAJOR.MINOR.PATCH format, for example 0.1.0.
EOF
}

fail() {
	printf '%s: %s\n' "$program_name installer" "$1" >&2
	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

cleanup() {
	if [ -n "$temporary_directory" ] && [ -d "$temporary_directory" ]; then
		rm -rf "$temporary_directory"
	fi
}

valid_version() {
	printf '%s\n' "$1" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
}

latest_version() {
	latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "${releases_url}/latest") ||
		fail "could not resolve the latest release"
	latest_tag=${latest_url##*/}

	case "$latest_tag" in
	v*) resolved_version=${latest_tag#v} ;;
	*) fail "latest release URL does not end in a v-prefixed tag" ;;
	esac

	valid_version "$resolved_version" || fail "latest release is not a stable version: $latest_tag"
	printf '%s\n' "$resolved_version"
}

case "$#" in
0)
	version=""
	;;
1)
	case "$1" in
	-h | --help)
		usage
		exit 0
		;;
	*) version=$1 ;;
	esac
	;;
*)
	usage >&2
	exit 2
	;;
esac

for command_name in awk curl grep install mktemp tar uname; do
	require_command "$command_name"
done

[ -n "${HOME:-}" ] || fail "HOME is not set"
case "$HOME" in
/*) ;;
*) fail "HOME must be an absolute path" ;;
esac

system_name=$(uname -s)
machine_name=$(uname -m)

case "${system_name}:${machine_name}" in
Linux:x86_64 | Linux:amd64)
	platform="linux"
	architecture="amd64"
	config_root=${XDG_CONFIG_HOME:-"$HOME/.config"}
	case "$config_root" in
	/*) ;;
	*) fail "XDG_CONFIG_HOME must be an absolute path" ;;
	esac
	;;
Darwin:arm64 | Darwin:aarch64)
	platform="darwin"
	architecture="arm64"
	config_root="$HOME/Library/Application Support"
	;;
*) fail "unsupported platform: ${system_name}/${machine_name}" ;;
esac

if [ -z "$version" ]; then
	version=$(latest_version)
else
	valid_version "$version" || fail "version must use stable MAJOR.MINOR.PATCH format"
fi

if command -v sha256sum >/dev/null 2>&1; then
	checksum_command="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
	checksum_command="shasum"
else
	fail "required checksum command not found: sha256sum or shasum"
fi

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/skycrate-install.XXXXXX") ||
	fail "could not create a temporary directory"
trap cleanup 0
trap 'exit 1' HUP INT TERM

archive_name="skycrate_${version}_${platform}_${architecture}.tar.gz"
release_download_url="${releases_url}/download/v${version}"
archive_path="${temporary_directory}/${archive_name}"
checksums_path="${temporary_directory}/SHA256SUMS"

printf 'Downloading Skycrate %s for %s/%s...\n' "$version" "$platform" "$architecture"
curl -fsSL --retry 3 -o "$archive_path" "${release_download_url}/${archive_name}" ||
	fail "could not download $archive_name"
curl -fsSL --retry 3 -o "$checksums_path" "${release_download_url}/SHA256SUMS" ||
	fail "could not download SHA256SUMS"

expected_checksum=$(awk -v archive="$archive_name" '$2 == archive { print $1; exit }' "$checksums_path")
[ -n "$expected_checksum" ] || fail "SHA256SUMS does not contain $archive_name"

case "$checksum_command" in
sha256sum) actual_checksum=$(sha256sum "$archive_path" | awk '{ print $1 }') ;;
shasum) actual_checksum=$(shasum -a 256 "$archive_path" | awk '{ print $1 }') ;;
esac

[ "$actual_checksum" = "$expected_checksum" ] || fail "checksum verification failed for $archive_name"

extraction_directory="${temporary_directory}/archive"
mkdir -p "$extraction_directory"
tar -xzf "$archive_path" -C "$extraction_directory" skycrate ||
	fail "could not extract skycrate from $archive_name"
[ -f "$extraction_directory/skycrate" ] || fail "archive does not contain the skycrate binary"

binary_directory="$HOME/.local/bin"
binary_path="$binary_directory/skycrate"
mkdir -p "$binary_directory"
install -m 0755 "$extraction_directory/skycrate" "$binary_path" ||
	fail "could not install skycrate to $binary_path"

config_directory="$config_root/skycrate"
config_path="$config_directory/config.yaml"
mkdir -p "$config_directory"

config_created="false"
if [ ! -e "$config_path" ] && [ ! -L "$config_path" ]; then
	if (
		umask 077
		set -C
		cat >"$config_path" <<'EOF'
bucket: skycrate-storage
region: us-east-1

categories:
  backup:
    tier: archive
  recordings:
    tier: cold
  documents:
    tier: instant
EOF
	) 2>/dev/null; then
		config_created="true"
	elif [ ! -e "$config_path" ] && [ ! -L "$config_path" ]; then
		fail "could not create default configuration at $config_path"
	fi
fi

printf 'Installed Skycrate %s to %s\n' "$version" "$binary_path"
if [ "$config_created" = "true" ]; then
	printf 'Created default configuration at %s\n' "$config_path"
else
	printf 'Preserved existing configuration at %s\n' "$config_path"
fi

case ":${PATH:-}:" in
*":${binary_directory}:"*) ;;
*) printf 'Warning: add %s to PATH to run skycrate directly.\n' "$binary_directory" >&2 ;;
esac

#!/usr/bin/env bash
# Install Painless on Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/heisdezz/painless/main/scripts/linux-install.sh | sh
#
# The script finds the newest Painless release and installs it the way the
# host supports:
#
#   * Debian/Ubuntu - the .deb with apt (through sudo), so dependencies are
#     resolved and the app is registered system-wide.
#   * Arch Linux    - the .tar.gz into ~/.local via the release's install.sh,
#     which also registers the desktop entry.
#   * anything else - the same archive into ~/.local.
#
# Options:
#
#   --version vX.Y.Z   install that release instead of the newest
#   --method METHOD    force deb or archive (default: detected)
#   --dry-run          print what would happen without changing anything
#   --help             show this help
#
# The archive install goes to ~/.local/<app>.app with a command in
# ~/.local/bin, which is where the app can update itself.
#
# Exit codes: 0 success, 1 failure, 2 usage error, 3 unsupported platform.

set -euo pipefail

REPO="heisdezz/painless"
APP_NAME="Painless"
SLUG="painless"

VERSION=""
METHOD=""
DRY_RUN=0

usage() {
  sed -n '2,/^$/p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || echo "Install $APP_NAME on Linux."
  exit "${1:-0}"
}

die() {
  printf '%s: error: %s\n' "$0" "$*" >&2
  exit 1
}

step() {
  printf '\n=> %s\n' "$*"
}

note() {
  printf '  %s\n' "$*"
}

warn() {
  printf '%s: warning: %s\n' "$0" "$*" >&2
}

run() {
  if [ "$DRY_RUN" -eq 1 ]; then
    printf '  would run: %s\n' "$*"
    return 0
  fi
  "$@"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      [ $# -ge 2 ] || die "--version needs a value"
      VERSION="$2"
      shift 2
      ;;
    --version=*) VERSION="${1#*=}"; shift ;;
    --method)
      [ $# -ge 2 ] || die "--method needs a value (deb or archive)"
      METHOD="$2"
      shift 2
      ;;
    --method=*) METHOD="${1#*=}"; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage 0 ;;
    *) die "unknown option: $1 (try --help)" ;;
  esac
done

case "$METHOD" in
  ""|deb|archive) ;;
  *) die "--method must be deb or archive, got '$METHOD'" ;;
esac

# --- checks -----------------------------------------------------------------

need() {
  command -v "$1" >/dev/null 2>&1 || die "this needs '$1' on PATH"
}

need curl
[ "$DRY_RUN" -eq 1 ] || need tar

[ "$(uname -s)" = "Linux" ] || die "$APP_NAME installs on Linux only"

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  *)
    printf '%s: error: no %s build for %s\n' "$0" "$APP_NAME" "$(uname -m)" >&2
    printf '  Releases are built for linux/amd64 only.\n' >&2
    exit 3
    ;;
esac

# --- find the release -------------------------------------------------------

api="https://api.github.com/repos/$REPO/releases"
if [ -n "$VERSION" ]; then
  case "$VERSION" in
    v*) tag="$VERSION" ;;
    *) tag="v$VERSION" ;;
  esac
  release_url="$api/tags/$tag"
else
  tag=""
  release_url="$api/latest"
fi

step "Finding the ${tag:-newest} $APP_NAME release"
if ! release_json="$(curl -fsSL "$release_url" 2>/dev/null)"; then
  if [ -n "$tag" ]; then
    die "release $tag not found in $REPO"
  fi
  die "no $APP_NAME releases are published in $REPO yet"
fi
[ -n "$release_json" ] || die "the GitHub API returned nothing for $REPO"

# The tag names the assets, so read it back from the release itself.
if [ -z "$tag" ]; then
  tag="$(printf '%s' "$release_json" | tr ',' '\n' |
    sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  [ -n "$tag" ] || die "could not read the release tag"
fi
version="${tag#v}"
note "$APP_NAME $version ($tag)"

asset_url() {
  # An asset the release lacks finds nothing, which is not an error here.
  printf '%s' "$release_json" | tr ',' '\n' |
    sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
    { grep -F "/$1" || true; } | head -n1
}

archive_name="$SLUG-$version-linux-$ARCH.tar.gz"
deb_name="${SLUG}_${version}_${ARCH}.deb"
archive_url="$(asset_url "$archive_name")"
deb_url="$(asset_url "$deb_name")"
installer_url="$(asset_url "install.sh")"

# --- pick how to install ----------------------------------------------------

have_root() {
  if [ "$(id -u)" -eq 0 ]; then
    return 0
  fi
  command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null
}

as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

detect_method() {
  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    case " ${ID:-} ${ID_LIKE:-} " in
      *debian*|*ubuntu*) printf 'deb'; return ;;
      *arch*) printf 'archive'; return ;;
    esac
  fi
  if command -v dpkg >/dev/null 2>&1; then
    printf 'deb'
    return
  fi
  printf 'archive'
}

[ -n "$METHOD" ] || METHOD="$(detect_method)"

# Earlier releases carried no .deb; the archive installs everywhere.
if [ "$METHOD" = "deb" ] && [ -z "$deb_url" ]; then
  warn "release $tag has no $deb_name, installing from the archive instead"
  METHOD="archive"
fi

# The .deb is installed by root. Without sudo, the user-scoped archive still
# works, so fall back rather than fail.
if [ "$METHOD" = "deb" ] && ! have_root; then
  warn "no sudo available, installing into ~/.local instead"
  METHOD="archive"
fi

# The release's install.sh installs for the current user, and refuses to run
# as root; the .deb is the root path.
if [ "$METHOD" = "archive" ] && [ "$(id -u)" -eq 0 ]; then
  die "run this as the user to install $APP_NAME for, not as root (or use --method deb)"
fi

# --- install ----------------------------------------------------------------

workdir="$(mktemp -d "${TMPDIR:-/tmp}/painless-install.XXXXXXXX")"
trap 'rm -rf "$workdir"' EXIT INT TERM

fetch() {
  url="$1"
  out="$2"
  name="$3"
  [ -n "$url" ] || die "release $tag has no $name"
  note "downloading $name"
  curl -fsSL --retry 3 --proto '=https' --tlsv1.2 -o "$out" "$url"
}

# link_into_path reports, or fixes, whether ~/.local/bin is on PATH.
link_into_path() {
  case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) note "Add ~/.local/bin to PATH to run $SLUG from a shell" ;;
  esac
}

# report_run says how to start the app from the command installed at link.
report_run() {
  case ":$PATH:" in
    *":$HOME/.local/bin:"*) note "Run it as: $SLUG" ;;
    *) note "Run it as: $1" ;;
  esac
}

install_deb() {
  step "Installing the Debian package"
  fetch "$deb_url" "$workdir/$deb_name" "$deb_name"

  if command -v apt-get >/dev/null 2>&1; then
    # apt resolves the package's dependencies; plain dpkg would leave the
    # app uninstalled-but-configured when one is missing.
    note "installing with apt (needs root)"
    if [ "$DRY_RUN" -eq 1 ]; then
      printf '  would run: apt-get install -y ./%s\n' "$deb_name"
    else
      as_root apt-get install -y "$workdir/$deb_name" </dev/null ||
        as_root apt-get install -y --fix-broken </dev/null
    fi
  else
    note "installing with dpkg"
    run as_root dpkg -i "$workdir/$deb_name"
  fi

  printf '\nInstalled %s %s\n' "$APP_NAME" "$version"
  note "Run it as: $SLUG (or from the applications menu)"
}

install_archive() {
  step "Installing into ~/.local"

  # install.sh is published beside the archive rather than inside it: the
  # tarball holds only the app and its desktop files. The script lays the app
  # out in ~/.local, links its commands into ~/.local/bin and registers the
  # desktop entry.
  [ -n "$installer_url" ] || die "release $tag has no install.sh"
  fetch "$installer_url" "$workdir/install.sh" "install.sh"
  fetch "$archive_url" "$workdir/$archive_name" "$archive_name"
  installer="$workdir/install.sh"

  if [ "$DRY_RUN" -eq 1 ]; then
    printf '  would run: sh %s %s\n' "$installer" "$workdir/$archive_name"
  else
    sh "$installer" "$workdir/$archive_name" </dev/null
  fi

  printf '\nInstalled %s %s\n' "$APP_NAME" "$version"
  link_into_path

  # The release's install.sh registers the app and its menu entry but, as
  # built today, links no command. Add one so the app runs by name.
  app_bin="$HOME/.local/$SLUG.app/$SLUG"
  link="$HOME/.local/bin/$SLUG"
  if [ -e "$link" ] || [ -L "$link" ]; then
    if [ "$(readlink "$link" 2>/dev/null)" != "$app_bin" ]; then
      note "Leaving $link alone: it is not $APP_NAME's"
      report_run "$link"
      return
    fi
  elif [ "$DRY_RUN" -eq 1 ]; then
    printf '  would link: %s -> %s\n' "$link" "$app_bin"
  else
    mkdir -p "$HOME/.local/bin"
    ln -sf "$app_bin" "$link"
  fi
  report_run "$link"
}

if [ "$METHOD" = "deb" ]; then
  install_deb
else
  install_archive
fi

# Painless

Painless is a native desktop media organizer for external drives. It indexes
photos and videos into a portable SQLite library that lives on the drive, so
the catalog travels with the media. The app scans and deduplicates files,
caches thumbnails, organizes albums, and opens media in a dedicated viewer.

## Install

### Linux

```sh
curl -fsSL https://raw.githubusercontent.com/heisdezz/painless/main/scripts/linux-install.sh | sh
```

The script finds the newest release and picks the install method for the host:

- **Debian/Ubuntu:** installs the `.deb` with `apt` (through `sudo`), so
  dependencies are resolved and the app is registered system-wide.
- **Arch Linux and other distributions:** installs the `.tar.gz` into
  `~/.local` using the archive's `install.sh`. No root is needed.

Useful options:

```sh
./linux-install.sh --version v1.2.3   # install a specific release
./linux-install.sh --method archive   # force the user-scoped archive
./linux-install.sh --dry-run          # show what would be installed
```

Only `linux/amd64` builds are published at the moment. Painless uses native
(MyGo/Lobe) widgets; the installer does not check for WebKitGTK.

### macOS and Windows

CI publishes macOS (universal) and Windows (x64) builds, but there is no
installer script for them yet. Download the installer from the release page
or build from source.

## Build from source

```sh
go tool mygo build
```

The build output is written to `build/` as configured in `mygo.json`.

## Development

```sh
CGO_ENABLED=0 go test ./...
go test -race ./internal/storage/... ./internal/state/...
go tool mygo vet .
gofmt -s -w .
```

Contributions follow [commit.md](commit.md) (Conventional Commits). The
shared task registry and multi-agent rules live in
[docs/implementation/progress.md](docs/implementation/progress.md).

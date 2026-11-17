# Logic Feature 06: Mobile Media Playback

## Scope

The native desktop viewer opens files on the active drive directly. Photos are decoded locally, and the video player passes a local file path to ffmpeg. Desktop playback does not start or depend on an HTTP server.

Mobile companions on the local network use the mobile sync server (default port `51790`). There is no separate streaming listener. The mobile server owns authentication, CORS, and listener lifecycle; the playback package only provides handlers.

## Endpoints

| Endpoint                             | Method        | Description                                                                                                             |
| ------------------------------------ | ------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `/stream?path=<drive-relative path>` | `GET`, `HEAD` | Serves a cataloged file from disk. `http.ServeContent` handles byte-range requests (`206 Partial Content`) for seeking. |
| `/thumbnail?mediaId=<id>`            | `GET`, `HEAD` | Serves the item's cached WebP thumbnail when thumbnailing is configured.                                                |

Both endpoints follow the mobile server's `ProtectReads` setting. When it is enabled, requests must include the pairing token as an `Authorization: Bearer` credential. Use `/api/ping` and `/api/status` for reachability and status.

Playback validates the requested relative path, looks it up in the media catalog, and checks that the resolved file stays inside the active drive root, including when symlinks are present. The server does not expose a directory tree. Thumbnails are addressed by media ID, so responses are marked private and must be revalidated before reuse.

## Implementation

- `internal/server/stream/routes.go`: Playback route registration and catalog interfaces.
- `internal/server/stream/handlers.go`: Range serving and thumbnail responses.
- `internal/server/stream/security.go`: Drive-root path checks.
- `internal/server/mobile/server.go`: Registers playback routes on the existing mobile listener.

## Acceptance Criteria

- [ ] Desktop playback opens local files without starting an HTTP listener.
- [ ] Mobile playback supports `GET`, `HEAD`, and byte-range seeking on the mobile sync listener.
- [ ] Playback rejects paths outside the active drive root and does not serve uncataloged files.
- [ ] Playback and thumbnail requests require a valid token when protected reads are enabled.

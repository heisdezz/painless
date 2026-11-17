# Logic Feature 07: Mobile Companion Sync REST & Upload API

## 1. Overview

The mobile sync service provides an authenticated REST and media upload API allowing mobile companion apps (e.g., iOS/Android apps on the local Wi-Fi network) to browse the library, stream media, and upload new photos/videos directly onto the external drive.

---

## 2. Security & Authentication

1. **Write Token Authentication**:
   - Write actions (`POST`, `PATCH`, `DELETE`) require an `Authorization: Bearer <token>` header.
   - Tokens are validated using constant-time comparison (`crypto/subtle.ConstantTimeCompare`) to prevent timing side-channel attacks.
   - Read actions can optionally be open or token-protected based on user settings.
2. **Local IP & Discovery**:
   - Computes local LAN IPv4 address (e.g. `192.168.1.150:51790`) to generate QR codes and pairing instructions in the UI.

---

## 3. API Endpoints

### Discovery & Status

- `GET /api/status`: Returns server health, active drive name, and capacity.
- `GET /api/ping`: Connection test.

### Media & Albums Read/Write

- `GET /api/albums`: Lists all albums with media counts.
- `POST /api/albums`: Creates a new album (requires token).
- `GET /api/media`: Paginated media items with search/filter query parameters.
- `GET /api/media/:id`: Item details and metadata JSON.

### Media Playback

- `GET`, `HEAD /stream?path=<drive-relative path>`: Serves cataloged files from disk and supports HTTP byte ranges for seeking.
- `GET`, `HEAD /thumbnail?mediaId=<id>`: Serves WebP thumbnails when configured.
- Both routes use the same mobile sync listener and follow its read-auth policy. Native desktop playback opens local files directly.

### Media Upload

- `POST /api/upload`: Multipart form upload or binary stream.
  - Headers: `Authorization`, `X-File-Name`, `X-Album-ID` (optional).
  - Max upload size: 2 GB per file.
  - Flow:
    1. Stream file directly into a temporary file (`.upload_<uuid>.tmp`) on the external drive (not RAM).
    2. Compute SHA-256 hash on-the-fly.
    3. Detect MIME type.
    4. Move to `unknown/` or `albums/<album>/`.
    5. Register in `.media_library.db` and trigger async thumbnail generation.

---

## 4. File Structure

- `internal/server/mobile/server.go`: Server lifecycle, CORS middleware, router.
- `internal/server/mobile/auth.go`: Constant-time token verification.
- `internal/server/mobile/handlers_crud.go`: Album & media query handlers.
- `internal/server/mobile/handlers_upload.go`: Streaming multipart file receiver.

---

## 5. Acceptance Criteria

- [ ] Mobile companion app can list albums and stream media over local Wi-Fi.
- [ ] Multipart uploads stream directly to disk without bloating memory.
- [ ] Unauthorized write requests receive `401 Unauthorized`.
- [ ] Uploaded files are immediately cataloged in SQLite and scheduled for thumbnailing.

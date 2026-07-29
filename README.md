# gorest-media

[![CI](https://github.com/nicolasbonnici/gorest-media/actions/workflows/ci.yml/badge.svg)](https://github.com/nicolasbonnici/gorest-media/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nicolasbonnici/gorest-media.svg)](https://pkg.go.dev/github.com/nicolasbonnici/gorest-media)
[![Go Version](https://img.shields.io/github/go-mod/go-version/nicolasbonnici/gorest-media)](https://github.com/nicolasbonnici/gorest-media/blob/HEAD/go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A GoREST plugin that exposes files — images, videos, PDFs, CSV/XLSX spreadsheets, and any
other format — as a first-class REST resource. Uploaded bytes live on local disk by default
or on any CDN/object gateway, and the storage backend is swappable without touching the API.

## Features

- **Any file type.** Uploads are classified into a coarse `kind` (`image`, `video`, `audio`,
  `document`, `spreadsheet`, `archive`, `other`) from the *sniffed* MIME type, not the
  client-supplied header. New formats are onboarded through config, not code.
- **Pluggable storage.** Ships with `local` (disk) and `cdn` (HTTP `PUT`/`GET`/`DELETE`
  against any S3-compatible or signed-URL gateway) drivers. Register your own with
  `media.RegisterStorage("s3", ...)`.
- **Content integrity.** Every object records its byte size and a SHA-256 checksum.
- **Safe by construction.** MIME allow-list, size ceiling, and path-traversal-proof local keys.

## Endpoints

| Method | Path                    | Description                                   |
|--------|-------------------------|-----------------------------------------------|
| POST   | `/media`                | Multipart upload (`file` field, optional `name`) |
| GET    | `/media`                | List with pagination and `kind`/`mime_type` filters |
| GET    | `/media/:id`            | Metadata                                      |
| GET    | `/media/:id/download`   | Stream the file bytes                         |
| PUT    | `/media/:id`            | Rename (only the display name is mutable)     |
| DELETE | `/media/:id`            | Delete the row and the stored bytes           |

Uploads record the authenticated user when auth middleware is mounted, and work anonymously
otherwise.

## Configuration

```yaml
plugins:
  - name: media
    enabled: true
    config:
      storage_driver: local          # local | cdn | <custom>
      local_base_path: ./storage/media
      max_file_size: 52428800        # 50 MiB
      pagination_limit: 25
      max_pagination_limit: 100
      allowed_mime_types:            # empty = accept any type
        - image/
        - application/pdf
        - text/csv
      kind_overrides:                # classify new formats without code
        application/x-parquet: spreadsheet
```

For the CDN driver:

```yaml
      storage_driver: cdn
      cdn_upload_url: https://storage.example.com/bucket   # PUT/DELETE target
      cdn_public_url: https://cdn.example.com/bucket       # public read URL (defaults to upload URL)
      cdn_auth_header: "Bearer <token>"                    # optional, sent on writes
```

## Extending storage

```go
media.RegisterStorage("s3", func(cfg *media.Config) (media.Storage, error) {
    return newS3Backend(cfg) // implement media.Storage
})
```

Then set `storage_driver: s3`. The `Storage` interface is four methods —
`Save`, `Open`, `Delete`, `URL` — plus `Driver()`.

## Development

```bash
make test           # race + coverage
make lint
make audit
```

Tests run against in-memory SQLite; production targets Postgres or MySQL. The schema is
provided by the plugin's migrations (`migrations/`) across all three dialects.

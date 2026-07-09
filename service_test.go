package media

import (
	"bytes"
	"context"
	"mime/multipart"
	"testing"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest/database"

	_ "github.com/nicolasbonnici/gorest/database/sqlite"
)

func newTestDB(t *testing.T) database.Database {
	t.Helper()

	db, err := database.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(context.Background(), `
		CREATE TABLE media (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			storage_key TEXT NOT NULL,
			storage_driver TEXT NOT NULL,
			mime_type TEXT NOT NULL,
			kind TEXT NOT NULL,
			extension TEXT NOT NULL DEFAULT '',
			size INTEGER NOT NULL DEFAULT 0,
			checksum TEXT NOT NULL DEFAULT '',
			user_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME
		)`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func newTestService(t *testing.T) *MediaService {
	t.Helper()
	db := newTestDB(t)
	cfg := DefaultConfig()
	cfg.LocalBasePath = t.TempDir()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate config: %v", err)
	}
	storage, err := NewStorage(&cfg)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	return NewMediaService(db, &cfg, storage)
}

// fileHeader turns raw bytes into a *multipart.FileHeader by round-tripping
// through a multipart writer, mirroring exactly what Fiber hands the service.
func fileHeader(t *testing.T, field, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	_ = w.Close()

	r := multipart.NewReader(&buf, w.Boundary())
	form, err := r.ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	files := form.File[field]
	if len(files) == 0 {
		t.Fatal("no file parsed")
	}
	return files[0]
}

var pngBytes = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89,
}

func TestServiceUpload(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	uid := uuid.New()

	m, err := svc.Upload(ctx, fileHeader(t, "file", "avatar.png", pngBytes), "", uid)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if m.Kind != KindImage {
		t.Errorf("Kind = %q, want image", m.Kind)
	}
	if m.MimeType != "image/png" {
		t.Errorf("MimeType = %q, want image/png", m.MimeType)
	}
	if m.Name != "avatar.png" {
		t.Errorf("Name = %q, want avatar.png", m.Name)
	}
	if m.Extension != "png" {
		t.Errorf("Extension = %q, want png", m.Extension)
	}
	if m.Size != int64(len(pngBytes)) {
		t.Errorf("Size = %d, want %d", m.Size, len(pngBytes))
	}
	if m.Checksum == "" {
		t.Error("Checksum not set")
	}
	if m.UserID != uid {
		t.Errorf("UserID = %v, want %v", m.UserID, uid)
	}

	rc, err := svc.Open(ctx, m)
	if err != nil {
		t.Fatalf("Open stored file: %v", err)
	}
	_ = rc.Close()
}

func TestServiceUploadCustomName(t *testing.T) {
	svc := newTestService(t)
	m, err := svc.Upload(context.Background(), fileHeader(t, "file", "raw.png", pngBytes), "  Nice Photo  ", uuid.Nil)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if m.Name != "Nice Photo" {
		t.Errorf("Name = %q, want trimmed 'Nice Photo'", m.Name)
	}
}

func TestServiceUploadTooLarge(t *testing.T) {
	svc := newTestService(t)
	svc.config.MaxFileSize = 4

	_, err := svc.Upload(context.Background(), fileHeader(t, "file", "big.png", pngBytes), "", uuid.Nil)
	if err != ErrFileTooLarge {
		t.Errorf("error = %v, want ErrFileTooLarge", err)
	}
}

func TestServiceUploadMimeNotAllowed(t *testing.T) {
	svc := newTestService(t)
	svc.config.AllowedMimeTypes = []string{"video/"}

	_, err := svc.Upload(context.Background(), fileHeader(t, "file", "avatar.png", pngBytes), "", uuid.Nil)
	if err == nil {
		t.Fatal("expected mime rejection")
	}
	if !isNotAllowed(err) {
		t.Errorf("error = %v, want 'is not allowed'", err)
	}
}

func TestServiceRename(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	m, _ := svc.Upload(ctx, fileHeader(t, "file", "avatar.png", pngBytes), "", uuid.Nil)

	renamed, err := svc.Rename(ctx, m.ID, "new-name.png")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Name != "new-name.png" {
		t.Errorf("Name = %q, want new-name.png", renamed.Name)
	}
	if renamed.StorageKey != m.StorageKey || renamed.MimeType != m.MimeType {
		t.Error("Rename must preserve storage columns")
	}
	if renamed.UpdatedAt == nil {
		t.Error("UpdatedAt should be set after rename")
	}

	if _, err := svc.Rename(ctx, m.ID, "   "); err == nil {
		t.Error("blank rename should fail")
	}
}

func TestServiceDelete(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	m, _ := svc.Upload(ctx, fileHeader(t, "file", "avatar.png", pngBytes), "", uuid.Nil)

	if err := svc.Delete(ctx, m); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.GetByID(ctx, m.ID); err == nil {
		t.Error("expected row to be gone")
	}
	if _, err := svc.Open(ctx, m); err == nil {
		t.Error("expected stored bytes to be gone")
	}
}

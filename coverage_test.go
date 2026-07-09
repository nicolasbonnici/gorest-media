package media

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	authcontext "github.com/nicolasbonnici/gorest/auth/context"
)

func TestUploadRecordsAuthenticatedUser(t *testing.T) {
	svc := newTestService(t)
	userID := uuid.New()

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		authcontext.SetUserID(c, userID.String())
		return c.Next()
	})
	RegisterRoutes(app, svc.db, svc.config, svc)

	resp, err := app.Test(uploadRequest(t, "avatar.png", pngBytes, ""))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	created := decodeMedia(t, resp)
	if created.UserID != userID {
		t.Errorf("UserID = %v, want %v", created.UserID, userID)
	}
}

func TestUploadWithMalformedUserIDStaysAnonymous(t *testing.T) {
	svc := newTestService(t)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		authcontext.SetUserID(c, "not-a-uuid")
		return c.Next()
	})
	RegisterRoutes(app, svc.db, svc.config, svc)

	resp, err := app.Test(uploadRequest(t, "avatar.png", pngBytes, ""))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if created := decodeMedia(t, resp); created.UserID != uuid.Nil {
		t.Errorf("UserID = %v, want nil", created.UserID)
	}
}

func TestNewLocalStorageRejectsUncreatablePath(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// A regular file cannot host a subdirectory, so MkdirAll must fail.
	if _, err := NewLocalStorage(filepath.Join(f, "sub")); err == nil {
		t.Error("expected error creating storage under a file")
	}
}

func TestCDNStorageNetworkErrors(t *testing.T) {
	s := NewCDNStorage("http://127.0.0.1:0", "", "")
	ctx := context.Background()
	if err := s.Save(ctx, "k", nil, 0, ""); err == nil {
		t.Error("Save to dead endpoint should fail")
	}
	if _, err := s.Open(ctx, "k"); err == nil {
		t.Error("Open from dead endpoint should fail")
	}
	if err := s.Delete(ctx, "k"); err == nil {
		t.Error("Delete on dead endpoint should fail")
	}
}

func TestUploadDerivesExtensionFromContent(t *testing.T) {
	svc := newTestService(t)
	m, err := svc.Upload(context.Background(), fileHeader(t, "file", "noextension", pngBytes), "", uuid.Nil)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if m.Extension != "png" {
		t.Errorf("Extension = %q, want png (sniffed)", m.Extension)
	}
}

func TestDownloadMissingStorageFile(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	m, _ := svc.Upload(ctx, fileHeader(t, "file", "avatar.png", pngBytes), "", uuid.Nil)

	// Remove the underlying bytes but keep the row, then the download route
	// must report the missing object rather than panicking.
	if err := svc.Storage().Delete(ctx, m.StorageKey); err != nil {
		t.Fatalf("delete stored bytes: %v", err)
	}

	app := fiber.New()
	RegisterRoutes(app, svc.db, svc.config, svc)
	resp, _ := app.Test(httptestNewRequest(http.MethodGet, "/media/"+m.ID.String()+"/download", nil))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDownloadUnknownID(t *testing.T) {
	app, _ := setupApp(t)
	resp, _ := app.Test(httptestNewRequest(http.MethodGet, "/media/"+uuid.New().String()+"/download", nil))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestNumericCoercionHelpers(t *testing.T) {
	if v, ok := toInt(float64(7)); !ok || v != 7 {
		t.Errorf("toInt(float64) = %d,%v", v, ok)
	}
	if v, ok := toInt(int64(9)); !ok || v != 9 {
		t.Errorf("toInt(int64) = %d,%v", v, ok)
	}
	if _, ok := toInt("nope"); ok {
		t.Error("toInt(string) should fail")
	}
	if v, ok := toInt64(float64(3)); !ok || v != 3 {
		t.Errorf("toInt64(float64) = %d,%v", v, ok)
	}
	if v, ok := toInt64(int(4)); !ok || v != 4 {
		t.Errorf("toInt64(int) = %d,%v", v, ok)
	}
	if _, ok := toInt64(true); ok {
		t.Error("toInt64(bool) should fail")
	}
	if v, ok := toStringSlice([]string{"a"}); !ok || len(v) != 1 {
		t.Errorf("toStringSlice([]string) = %v,%v", v, ok)
	}
	if _, ok := toStringSlice(42); ok {
		t.Error("toStringSlice(int) should fail")
	}
	if v, ok := toStringMap(map[string]string{"k": "v"}); !ok || v["k"] != "v" {
		t.Errorf("toStringMap = %v,%v", v, ok)
	}
	if _, ok := toStringMap(42); ok {
		t.Error("toStringMap(int) should fail")
	}
}

package media

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	pluginmigrations "github.com/nicolasbonnici/gorest-media/migrations"
	"github.com/nicolasbonnici/gorest/database"

	_ "github.com/nicolasbonnici/gorest/database/sqlite"
)

func setupApp(t *testing.T) (*fiber.App, *MediaService) {
	t.Helper()

	db, err := database.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	migs, err := pluginmigrations.GetMigrations().Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	for _, m := range migs {
		if err := m.ExecuteUp(ctx, db); err != nil {
			t.Fatalf("apply migration %s: %v", m.FullName(), err)
		}
	}

	cfg := DefaultConfig()
	cfg.LocalBasePath = t.TempDir()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	storage, err := NewStorage(&cfg)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	svc := NewMediaService(db, &cfg, storage)

	app := fiber.New()
	RegisterRoutes(app, db, &cfg, svc)
	return app, svc
}

func uploadRequest(t *testing.T, filename string, content []byte, extraName string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	_, _ = fw.Write(content)
	if extraName != "" {
		_ = w.WriteField("name", extraName)
	}
	_ = w.Close()

	req := httptestNewRequest(http.MethodPost, "/media", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func httptestNewRequest(method, target string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, target, body)
	return req
}

func decodeMedia(t *testing.T, resp *http.Response) MediaResponseDTO {
	t.Helper()
	var dto MediaResponseDTO
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		t.Fatalf("decode media: %v", err)
	}
	return dto
}

func TestHTTPUploadAndLifecycle(t *testing.T) {
	app, _ := setupApp(t)

	resp, err := app.Test(uploadRequest(t, "avatar.png", pngBytes, "Profile Picture"), fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, want 201", resp.StatusCode)
	}
	created := decodeMedia(t, resp)
	if created.Kind != KindImage || created.Name != "Profile Picture" {
		t.Errorf("unexpected media: %+v", created)
	}
	if created.URL != "/media/"+created.ID.String()+"/download" {
		t.Errorf("URL = %q", created.URL)
	}

	getResp, _ := app.Test(httptestNewRequest(http.MethodGet, "/media/"+created.ID.String(), nil))
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", getResp.StatusCode)
	}

	dlResp, err := app.Test(httptestNewRequest(http.MethodGet, "/media/"+created.ID.String()+"/download", nil))
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d", dlResp.StatusCode)
	}
	if ct := dlResp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("download Content-Type = %q, want image/png", ct)
	}
	body, _ := io.ReadAll(dlResp.Body)
	if !bytes.Equal(body, pngBytes) {
		t.Error("downloaded bytes differ from upload")
	}

	listResp, _ := app.Test(httptestNewRequest(http.MethodGet, "/media", nil))
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", listResp.StatusCode)
	}
	var list struct {
		Member []MediaResponseDTO `json:"hydra:member"`
	}
	_ = json.NewDecoder(listResp.Body).Decode(&list)
	if len(list.Member) != 1 {
		t.Errorf("list returned %d items, want 1", len(list.Member))
	}

	renameBody := bytes.NewBufferString(`{"name":"renamed.png"}`)
	putReq := httptestNewRequest(http.MethodPut, "/media/"+created.ID.String(), renameBody)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, _ := app.Test(putReq)
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d", putResp.StatusCode)
	}
	if updated := decodeMedia(t, putResp); updated.Name != "renamed.png" {
		t.Errorf("renamed to %q", updated.Name)
	}

	delResp, _ := app.Test(httptestNewRequest(http.MethodDelete, "/media/"+created.ID.String(), nil))
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", delResp.StatusCode)
	}

	gone, _ := app.Test(httptestNewRequest(http.MethodGet, "/media/"+created.ID.String(), nil))
	if gone.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", gone.StatusCode)
	}
}

func TestHTTPUploadMissingFile(t *testing.T) {
	app, _ := setupApp(t)
	req := httptestNewRequest(http.MethodPost, "/media", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHTTPUploadTooLarge(t *testing.T) {
	app, svc := setupApp(t)
	svc.config.MaxFileSize = 4

	resp, _ := app.Test(uploadRequest(t, "big.png", pngBytes, ""))
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", resp.StatusCode)
	}
}

func TestHTTPUploadUnsupportedType(t *testing.T) {
	app, svc := setupApp(t)
	svc.config.AllowedMimeTypes = []string{"video/"}

	resp, _ := app.Test(uploadRequest(t, "avatar.png", pngBytes, ""))
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", resp.StatusCode)
	}
}

func TestHTTPInvalidID(t *testing.T) {
	app, _ := setupApp(t)
	resp, _ := app.Test(httptestNewRequest(http.MethodGet, "/media/not-a-uuid/download", nil))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("download invalid id status = %d, want 400", resp.StatusCode)
	}

	putReq := httptestNewRequest(http.MethodPut, "/media/not-a-uuid", bytes.NewBufferString(`{"name":"x"}`))
	putReq.Header.Set("Content-Type", "application/json")
	putResp, _ := app.Test(putReq)
	if putResp.StatusCode != http.StatusBadRequest {
		t.Errorf("update invalid id status = %d, want 400", putResp.StatusCode)
	}

	delResp, _ := app.Test(httptestNewRequest(http.MethodDelete, "/media/not-a-uuid", nil))
	if delResp.StatusCode != http.StatusBadRequest {
		t.Errorf("delete invalid id status = %d, want 400", delResp.StatusCode)
	}
}

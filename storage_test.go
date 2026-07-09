package media

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLocalStorageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStorage(dir)
	if err != nil {
		t.Fatalf("NewLocalStorage: %v", err)
	}
	if s.Driver() != DriverLocal {
		t.Errorf("Driver = %q", s.Driver())
	}

	ctx := context.Background()
	key := "2026/07/file.txt"
	if err := s.Save(ctx, key, strings.NewReader("hello world"), 11, "text/plain"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rc, err := s.Open(ctx, key)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(data) != "hello world" {
		t.Errorf("read %q, want %q", data, "hello world")
	}

	if s.URL(key) != "" {
		t.Errorf("local URL should be empty, got %q", s.URL(key))
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Open(ctx, key); err == nil {
		t.Error("expected error opening deleted file")
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("deleting missing file should be nil, got %v", err)
	}
}

func TestLocalStoragePathTraversal(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocalStorage(dir)
	ctx := context.Background()

	if err := s.Save(ctx, "../escape.txt", strings.NewReader("x"), 1, ""); err == nil {
		t.Fatal("expected traversal key to be rejected")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.txt")); err == nil {
		t.Fatal("file escaped the base directory")
	}
}

func TestCDNStorageRoundTrip(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			if r.Header.Get("Authorization") != "Bearer token" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			body, _ := io.ReadAll(r.Body)
			objects[r.URL.Path] = body
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			body, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(body)
		case http.MethodDelete:
			delete(objects, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	s := NewCDNStorage(srv.URL, "", "Bearer token")
	if s.Driver() != DriverCDN {
		t.Errorf("Driver = %q", s.Driver())
	}

	ctx := context.Background()
	key := "a/b/photo.png"
	if err := s.Save(ctx, key, strings.NewReader("bytes"), 5, "image/png"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if want := srv.URL + "/a/b/photo.png"; s.URL(key) != want {
		t.Errorf("URL = %q, want %q", s.URL(key), want)
	}

	rc, err := s.Open(ctx, key)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(data) != "bytes" {
		t.Errorf("read %q, want bytes", data)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Open(ctx, key); err == nil {
		t.Error("expected fetch of deleted object to fail")
	}
}

func TestCDNStorageUploadRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	s := NewCDNStorage(srv.URL, "", "")
	err := s.Save(context.Background(), "k", strings.NewReader("x"), 1, "")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 error, got %v", err)
	}
}

func TestCDNStorageDeleteMissingIsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := NewCDNStorage(srv.URL, "", "")
	if err := s.Delete(context.Background(), "missing"); err != nil {
		t.Errorf("delete of missing object should be nil, got %v", err)
	}
}

func TestNewStorageFromConfig(t *testing.T) {
	local, err := NewStorage(&Config{StorageDriver: DriverLocal, LocalBasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("local: %v", err)
	}
	if local.Driver() != DriverLocal {
		t.Errorf("Driver = %q", local.Driver())
	}

	cdn, err := NewStorage(&Config{StorageDriver: DriverCDN, CDNUploadURL: "https://cdn.example.com"})
	if err != nil {
		t.Fatalf("cdn: %v", err)
	}
	if cdn.Driver() != DriverCDN {
		t.Errorf("Driver = %q", cdn.Driver())
	}

	if _, err := NewStorage(&Config{StorageDriver: "nope"}); err == nil {
		t.Error("expected error for unknown driver")
	}
}

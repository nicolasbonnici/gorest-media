package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Storage is the pluggable backend that holds the raw bytes of a media object.
// The database only ever stores a key; resolving that key to bytes (or a public
// URL) is entirely the backend's concern, which is what lets the same media
// resource live on local disk or a CDN without touching the HTTP layer.
type Storage interface {
	Driver() string
	Save(ctx context.Context, key string, r io.Reader, size int64, mimeType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	// URL returns a publicly reachable address for the object, or "" when the
	// backend has none and bytes must be streamed through the download route.
	URL(key string) string
}

type StorageFactory func(cfg *Config) (Storage, error)

var (
	storageFactoriesMu sync.RWMutex
	storageFactories   = map[string]StorageFactory{}
)

// RegisterStorage adds a storage driver under name, overwriting any existing
// registration. Call it from an init() to teach the plugin a new backend (e.g.
// an S3 client) without editing this package.
func RegisterStorage(name string, factory StorageFactory) {
	storageFactoriesMu.Lock()
	defer storageFactoriesMu.Unlock()
	storageFactories[name] = factory
}

func lookupStorageFactory(name string) (StorageFactory, bool) {
	storageFactoriesMu.RLock()
	defer storageFactoriesMu.RUnlock()
	f, ok := storageFactories[name]
	return f, ok
}

func NewStorage(cfg *Config) (Storage, error) {
	factory, ok := lookupStorageFactory(cfg.StorageDriver)
	if !ok {
		return nil, fmt.Errorf("unknown storage_driver: %s", cfg.StorageDriver)
	}
	return factory(cfg)
}

func init() {
	RegisterStorage(DriverLocal, func(cfg *Config) (Storage, error) {
		return NewLocalStorage(cfg.LocalBasePath)
	})
	RegisterStorage(DriverCDN, func(cfg *Config) (Storage, error) {
		return NewCDNStorage(cfg.CDNUploadURL, cfg.CDNPublicURL, cfg.CDNAuthHeader), nil
	})
}

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}
	return &LocalStorage{basePath: basePath}, nil
}

func (s *LocalStorage) Driver() string { return DriverLocal }

// resolve joins the key onto the base path while refusing any key that escapes
// it, so a crafted "../../etc/passwd" key can never read or clobber files
// outside the configured directory.
func (s *LocalStorage) resolve(key string) (string, error) {
	clean := filepath.Clean(key)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid storage key: %s", key)
	}
	return filepath.Join(s.basePath, clean), nil
}

func (s *LocalStorage) Save(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return f.Sync()
}

func (s *LocalStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *LocalStorage) URL(string) string { return "" }

// CDNStorage speaks plain HTTP to an object gateway: PUT to write, GET to read,
// DELETE to remove. It carries no vendor SDK, so it works against S3-compatible
// endpoints, a reverse proxy, or any signed-URL gateway the operator points it
// at.
type CDNStorage struct {
	uploadBase string
	publicBase string
	authHeader string
	client     *http.Client
}

func NewCDNStorage(uploadBase, publicBase, authHeader string) *CDNStorage {
	if publicBase == "" {
		publicBase = uploadBase
	}
	return &CDNStorage{
		uploadBase: strings.TrimRight(uploadBase, "/"),
		publicBase: strings.TrimRight(publicBase, "/"),
		authHeader: authHeader,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *CDNStorage) Driver() string { return DriverCDN }

func (s *CDNStorage) objectURL(base, key string) string {
	return base + "/" + strings.TrimLeft(key, "/")
}

func (s *CDNStorage) Save(ctx context.Context, key string, r io.Reader, size int64, mimeType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.objectURL(s.uploadBase, key), r)
	if err != nil {
		return err
	}
	if size > 0 {
		req.ContentLength = size
	}
	if mimeType != "" {
		req.Header.Set("Content-Type", mimeType)
	}
	if s.authHeader != "" {
		req.Header.Set("Authorization", s.authHeader)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cdn upload failed with status %d", resp.StatusCode)
	}
	return nil
}

func (s *CDNStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.objectURL(s.publicBase, key), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("cdn fetch failed with status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (s *CDNStorage) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.objectURL(s.uploadBase, key), nil)
	if err != nil {
		return err
	}
	if s.authHeader != "" {
		req.Header.Set("Authorization", s.authHeader)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cdn delete failed with status %d", resp.StatusCode)
	}
	return nil
}

func (s *CDNStorage) URL(key string) string {
	return s.objectURL(s.publicBase, key)
}

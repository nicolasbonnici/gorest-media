package media

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nicolasbonnici/gorest/database"
)

const (
	DriverLocal = "local"
	DriverCDN   = "cdn"
)

type Config struct {
	Database database.Database

	StorageDriver string `json:"storage_driver" yaml:"storage_driver"`

	LocalBasePath string `json:"local_base_path" yaml:"local_base_path"`

	// CDNUploadURL is the base URL objects are written to (HTTP PUT/DELETE).
	// CDNPublicURL is the base URL objects are read from; when empty it falls
	// back to CDNUploadURL. Splitting them supports gateways whose write and
	// read hosts differ (e.g. a signed-upload endpoint vs. a public CDN edge).
	CDNUploadURL string `json:"cdn_upload_url" yaml:"cdn_upload_url"`
	CDNPublicURL string `json:"cdn_public_url" yaml:"cdn_public_url"`
	// CDNAuthHeader is sent verbatim as the Authorization header on write
	// requests, letting the operator supply a static token without this plugin
	// pulling in a vendor SDK.
	CDNAuthHeader string `json:"cdn_auth_header" yaml:"cdn_auth_header"`

	MaxFileSize int64 `json:"max_file_size" yaml:"max_file_size"`

	// AllowedMimeTypes gates uploads by detected MIME type. An empty list
	// accepts any type; entries ending in "/" match a whole family (e.g.
	// "image/" allows image/png and image/jpeg alike).
	AllowedMimeTypes []string `json:"allowed_mime_types" yaml:"allowed_mime_types"`

	// KindOverrides maps a MIME type (exact, or a "family/" prefix) to a media
	// kind, layered over the built-in defaults. This is the extension point for
	// classifying new formats without a code change.
	KindOverrides map[string]string `json:"kind_overrides" yaml:"kind_overrides"`

	PaginationLimit    int `json:"pagination_limit" yaml:"pagination_limit"`
	MaxPaginationLimit int `json:"max_pagination_limit" yaml:"max_pagination_limit"`
}

func DefaultConfig() Config {
	return Config{
		StorageDriver:      DriverLocal,
		LocalBasePath:      "./storage/media",
		MaxFileSize:        50 << 20,
		AllowedMimeTypes:   nil,
		PaginationLimit:    25,
		MaxPaginationLimit: 100,
	}
}

func (c *Config) Validate() error {
	c.applyDefaults()

	switch c.StorageDriver {
	case DriverLocal:
		if strings.TrimSpace(c.LocalBasePath) == "" {
			return errors.New("local_base_path is required for the local storage driver")
		}
	case DriverCDN:
		if strings.TrimSpace(c.CDNUploadURL) == "" {
			return errors.New("cdn_upload_url is required for the cdn storage driver")
		}
	default:
		if _, ok := lookupStorageFactory(c.StorageDriver); !ok {
			return fmt.Errorf("unknown storage_driver: %s", c.StorageDriver)
		}
	}

	if c.MaxFileSize < 1 {
		return errors.New("max_file_size must be greater than 0")
	}

	for _, mt := range c.AllowedMimeTypes {
		if strings.TrimSpace(mt) == "" {
			return errors.New("allowed_mime_types cannot contain empty strings")
		}
	}

	return nil
}

func (c *Config) applyDefaults() {
	if c.StorageDriver == "" {
		c.StorageDriver = DriverLocal
	}
	if c.LocalBasePath == "" {
		c.LocalBasePath = "./storage/media"
	}
	if c.MaxFileSize == 0 {
		c.MaxFileSize = 50 << 20
	}
	if c.PaginationLimit <= 0 {
		c.PaginationLimit = 25
	}
	if c.MaxPaginationLimit <= 0 {
		c.MaxPaginationLimit = 100
	}
}

func (c *Config) IsAllowedMime(mime string) bool {
	if len(c.AllowedMimeTypes) == 0 {
		return true
	}
	for _, allowed := range c.AllowedMimeTypes {
		if strings.HasSuffix(allowed, "/") {
			if strings.HasPrefix(mime, allowed) {
				return true
			}
			continue
		}
		if allowed == mime {
			return true
		}
	}
	return false
}

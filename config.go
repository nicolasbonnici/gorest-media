package media

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
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

	// Reads stay public; uploads and mutations require one of WriteRoles or the
	// superuser role. Before v0.7 these routes carried no guard, so anyone
	// could upload to, or delete from, the storage backend anonymously.
	WriteRoles    []string            `json:"write_roles" yaml:"write_roles"`
	SuperuserRole string              `json:"superuser_role" yaml:"superuser_role"`
	RoleHierarchy map[string][]string `json:"role_hierarchy" yaml:"role_hierarchy"`

	// AuthMiddleware is supplied by the plugin loader when the host enables
	// auth. Without it nothing can present an identity, so the guards deny
	// every mutation rather than waving them through.
	AuthMiddleware fiber.Handler `json:"-" yaml:"-"`

	PaginationLimit    int `json:"pagination_limit" yaml:"pagination_limit"`
	MaxPaginationLimit int `json:"max_pagination_limit" yaml:"max_pagination_limit"`
}

// DefaultAllowedMimeTypes covers the formats this plugin classifies in kind.go
// and nothing else. It used to default to nil, which IsAllowedMime reads as
// "allow anything" - so a stock install accepted scripts and executables and
// stored them under a web root. Operators who genuinely want everything can
// still set allowed_mime_types to an empty list explicitly.
func DefaultAllowedMimeTypes() []string {
	return []string{
		// Raster formats are enumerated rather than allowing the whole "image/"
		// family, because that family includes image/svg+xml - an XML document
		// that carries <script> and executes when a browser renders it inline.
		"image/apng",
		"image/avif",
		"image/bmp",
		"image/gif",
		"image/jpeg",
		"image/png",
		"image/tiff",
		"image/webp",
		"image/x-icon",
		"video/",
		"audio/",
		"text/plain",
		"text/csv",
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/rtf",
		"application/csv",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/json",
		"application/zip",
		"application/gzip",
		"application/x-tar",
		"application/x-7z-compressed",
		"application/x-rar-compressed",
	}
}

func DefaultConfig() Config {
	return Config{
		StorageDriver:      DriverLocal,
		LocalBasePath:      "./storage/media",
		MaxFileSize:        50 << 20,
		AllowedMimeTypes:   DefaultAllowedMimeTypes(),
		PaginationLimit:    25,
		MaxPaginationLimit: 100,
		WriteRoles:         []string{"writer", "moderator"},
		SuperuserRole:      "admin",
		RoleHierarchy: map[string][]string{
			"moderator": {"writer"},
			"writer":    {"reader"},
		},
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
	// nil means "never configured" and gets the safe list; an explicitly empty
	// list is the operator saying they want every type, and is left alone.
	if c.AllowedMimeTypes == nil {
		c.AllowedMimeTypes = DefaultAllowedMimeTypes()
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

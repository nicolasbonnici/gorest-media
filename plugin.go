package media

import (
	"github.com/gofiber/fiber/v3"
	"github.com/nicolasbonnici/gorest-media/migrations"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/logger"
	"github.com/nicolasbonnici/gorest/plugin"
)

type MediaPlugin struct {
	config  Config
	db      database.Database
	storage Storage
	service *MediaService
}

func (p *MediaPlugin) GetService() *MediaService { return p.service }

func NewPlugin() plugin.Plugin {
	return &MediaPlugin{}
}

func (p *MediaPlugin) Name() string { return "media" }

func (p *MediaPlugin) Initialize(config map[string]any) error {
	p.config = DefaultConfig()

	if db, ok := config["database"].(database.Database); ok {
		p.db = db
		p.config.Database = db
	}

	applyStringConfig(config, "storage_driver", &p.config.StorageDriver)
	applyStringConfig(config, "local_base_path", &p.config.LocalBasePath)
	applyStringConfig(config, "cdn_upload_url", &p.config.CDNUploadURL)
	applyStringConfig(config, "cdn_public_url", &p.config.CDNPublicURL)
	applyStringConfig(config, "cdn_auth_header", &p.config.CDNAuthHeader)

	if maxSize, ok := toInt64(config["max_file_size"]); ok {
		p.config.MaxFileSize = maxSize
	}
	if limit, ok := toInt(config["pagination_limit"]); ok {
		p.config.PaginationLimit = limit
	}
	if maxLimit, ok := toInt(config["max_pagination_limit"]); ok {
		p.config.MaxPaginationLimit = maxLimit
	}
	if types, ok := toStringSlice(config["allowed_mime_types"]); ok {
		p.config.AllowedMimeTypes = types
	}
	if overrides, ok := toStringMap(config["kind_overrides"]); ok {
		p.config.KindOverrides = overrides
	}

	if err := p.config.Validate(); err != nil {
		logger.Log.Error("Invalid media plugin configuration", "error", err)
		return err
	}

	if p.db == nil {
		logger.Log.Warn("Media plugin initialized without database - endpoints will not be available")
		return nil
	}

	storage, err := NewStorage(&p.config)
	if err != nil {
		logger.Log.Error("Failed to initialize media storage backend", "error", err)
		return err
	}
	p.storage = storage
	p.service = NewMediaService(p.db, &p.config, storage)

	logger.Log.Info("Media plugin initialized", "storage_driver", p.config.StorageDriver)
	return nil
}

func (p *MediaPlugin) Handler() fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.Next()
	}
}

func (p *MediaPlugin) SetupEndpoints(router fiber.Router) error {
	if p.service == nil {
		logger.Log.Warn("Media plugin service not initialized, skipping endpoint registration")
		return nil
	}

	RegisterRoutes(router, p.db, &p.config, p.service)
	logger.Log.Info("Media plugin endpoints registered")
	return nil
}

func (p *MediaPlugin) MigrationSource() any {
	return migrations.GetMigrations()
}

func (p *MediaPlugin) MigrationDependencies() []string { return []string{} }

func (p *MediaPlugin) Dependencies() []string { return []string{} }

func (p *MediaPlugin) GetOpenAPIResources() []plugin.OpenAPIResource {
	return []plugin.OpenAPIResource{
		{
			Name:          "media",
			PluralName:    "media",
			BasePath:      "/media",
			Tags:          []string{"Media"},
			ResponseModel: MediaResponseDTO{},
			UpdateModel:   MediaUpdateDTO{},
			Description:   "Files (images, video, documents, spreadsheets, and any other format) stored locally or on a CDN",
		},
	}
}

func applyStringConfig(config map[string]any, key string, target *string) {
	if v, ok := config[key].(string); ok && v != "" {
		*target = v
	}
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

func toStringSlice(v any) ([]string, bool) {
	switch raw := v.(type) {
	case []string:
		return raw, true
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

func toStringMap(v any) (map[string]string, bool) {
	switch raw := v.(type) {
	case map[string]string:
		return raw, true
	case map[string]any:
		out := make(map[string]string, len(raw))
		for k, item := range raw {
			if s, ok := item.(string); ok {
				out[k] = s
			}
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

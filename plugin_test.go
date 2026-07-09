package media

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
	"github.com/nicolasbonnici/gorest/plugin"
)

func TestNewPlugin(t *testing.T) {
	p := NewPlugin()
	mp, ok := p.(*MediaPlugin)
	if !ok {
		t.Fatal("NewPlugin did not return *MediaPlugin")
	}
	if mp.Name() != "media" {
		t.Errorf("Name = %q, want media", mp.Name())
	}
	if mp.Handler() == nil {
		t.Error("Handler returned nil")
	}
}

func TestPluginInitializeDefaults(t *testing.T) {
	p := NewPlugin().(*MediaPlugin)
	if err := p.Initialize(map[string]any{}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if p.config.StorageDriver != DriverLocal {
		t.Errorf("StorageDriver = %q", p.config.StorageDriver)
	}
	if p.service != nil {
		t.Error("service should be nil without a database")
	}
}

func TestPluginInitializeCustomConfig(t *testing.T) {
	p := NewPlugin().(*MediaPlugin)
	err := p.Initialize(map[string]any{
		"storage_driver":       "cdn",
		"cdn_upload_url":       "https://cdn.example.com",
		"max_file_size":        int64(1024),
		"pagination_limit":     10,
		"max_pagination_limit": 50,
		"allowed_mime_types":   []any{"image/", "application/pdf"},
		"kind_overrides":       map[string]any{"application/octet-stream": "binary"},
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if p.config.StorageDriver != DriverCDN {
		t.Errorf("StorageDriver = %q", p.config.StorageDriver)
	}
	if p.config.MaxFileSize != 1024 || p.config.PaginationLimit != 10 {
		t.Errorf("numeric config not applied: %+v", p.config)
	}
	if len(p.config.AllowedMimeTypes) != 2 {
		t.Errorf("AllowedMimeTypes = %v", p.config.AllowedMimeTypes)
	}
	if p.config.KindOverrides["application/octet-stream"] != "binary" {
		t.Errorf("KindOverrides = %v", p.config.KindOverrides)
	}
}

func TestPluginInitializeInvalidConfig(t *testing.T) {
	p := NewPlugin().(*MediaPlugin)
	err := p.Initialize(map[string]any{"storage_driver": "cdn"}) // missing cdn_upload_url
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestPluginInitializeWithDatabase(t *testing.T) {
	p := NewPlugin().(*MediaPlugin)
	dir := t.TempDir()
	err := p.Initialize(map[string]any{
		"database":        database.Database(&mockDatabase{}),
		"local_base_path": dir,
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if p.service == nil {
		t.Fatal("service should be initialized with a database")
	}
	if p.GetService() == nil {
		t.Error("GetService returned nil")
	}
	if err := p.SetupEndpoints(fiber.New()); err != nil {
		t.Errorf("SetupEndpoints: %v", err)
	}
}

func TestPluginSetupEndpointsWithoutService(t *testing.T) {
	p := NewPlugin().(*MediaPlugin)
	_ = p.Initialize(map[string]any{})
	if err := p.SetupEndpoints(fiber.New()); err != nil {
		t.Errorf("SetupEndpoints should no-op without service, got %v", err)
	}
}

func TestPluginMigrationAndOpenAPI(t *testing.T) {
	p := NewPlugin().(*MediaPlugin)
	src, ok := p.MigrationSource().(migrations.MigrationSource)
	if !ok {
		t.Fatal("MigrationSource is not a migrations.MigrationSource")
	}
	migs, err := src.Migrations()
	if err != nil || len(migs) == 0 {
		t.Fatalf("migrations: %v (%d)", err, len(migs))
	}
	if len(p.Dependencies()) != 0 || len(p.MigrationDependencies()) != 0 {
		t.Error("expected no dependencies")
	}

	resources := p.GetOpenAPIResources()
	if len(resources) != 1 || resources[0].BasePath != "/media" {
		t.Errorf("unexpected OpenAPI resources: %+v", resources)
	}
}

type mockDatabase struct{}

func (m *mockDatabase) Connect(context.Context, string) error { return nil }
func (m *mockDatabase) Close() error                          { return nil }
func (m *mockDatabase) Ping(context.Context) error            { return nil }
func (m *mockDatabase) Query(context.Context, string, ...any) (database.Rows, error) {
	return nil, nil
}
func (m *mockDatabase) QueryRow(context.Context, string, ...any) database.Row { return nil }
func (m *mockDatabase) Exec(context.Context, string, ...any) (database.Result, error) {
	return nil, nil
}
func (m *mockDatabase) Begin(context.Context) (database.Tx, error) { return nil, nil }
func (m *mockDatabase) Dialect() database.Dialect                  { return nil }
func (m *mockDatabase) DriverName() string                         { return "mock" }
func (m *mockDatabase) Introspector() database.SchemaIntrospector  { return nil }

var _ plugin.Plugin = (*MediaPlugin)(nil)

package media

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.StorageDriver != DriverLocal {
		t.Errorf("StorageDriver = %q, want %q", cfg.StorageDriver, DriverLocal)
	}
	if cfg.MaxFileSize != 50<<20 {
		t.Errorf("MaxFileSize = %d, want %d", cfg.MaxFileSize, 50<<20)
	}
	if cfg.PaginationLimit != 25 || cfg.MaxPaginationLimit != 100 {
		t.Errorf("pagination = %d/%d, want 25/100", cfg.PaginationLimit, cfg.MaxPaginationLimit)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Config)
		wantError bool
	}{
		{"default local", func(*Config) {}, false},
		{"local without path", func(c *Config) { c.LocalBasePath = "" }, false}, // default reapplied
		{"cdn with url", func(c *Config) {
			c.StorageDriver = DriverCDN
			c.CDNUploadURL = "https://cdn.example.com"
		}, false},
		{"cdn without url", func(c *Config) { c.StorageDriver = DriverCDN }, true},
		{"unknown driver", func(c *Config) { c.StorageDriver = "ftp" }, true},
		{"zero max size stays defaulted", func(c *Config) { c.MaxFileSize = 0 }, false},
		{"negative max size", func(c *Config) { c.MaxFileSize = -1 }, true},
		{"empty allowed mime", func(c *Config) { c.AllowedMimeTypes = []string{""} }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestConfigValidateRegisteredCustomDriver(t *testing.T) {
	RegisterStorage("test-custom", func(*Config) (Storage, error) { return nil, nil })

	cfg := DefaultConfig()
	cfg.StorageDriver = "test-custom"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("custom driver should validate, got %v", err)
	}
}

func TestIsAllowedMime(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		mime    string
		want    bool
	}{
		{"empty allows all", nil, "image/png", true},
		{"exact match", []string{"image/png"}, "image/png", true},
		{"exact mismatch", []string{"image/png"}, "image/jpeg", false},
		{"family match", []string{"image/"}, "image/gif", true},
		{"family mismatch", []string{"image/"}, "video/mp4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{AllowedMimeTypes: tt.allowed}
			if got := cfg.IsAllowedMime(tt.mime); got != tt.want {
				t.Errorf("IsAllowedMime(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}

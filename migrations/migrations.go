package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func GetMigrations() migrations.MigrationSource {
	builder := migrations.NewMigrationBuilder("gorest-media")
	builder.Add("20260710000001000", "create_media_table", upMedia, downMedia)
	return builder.Build()
}

func upMedia(ctx context.Context, db database.Database) error {
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `CREATE TABLE IF NOT EXISTS media (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(512) NOT NULL,
			storage_key VARCHAR(1024) NOT NULL,
			storage_driver VARCHAR(64) NOT NULL,
			mime_type VARCHAR(255) NOT NULL,
			kind VARCHAR(32) NOT NULL,
			extension VARCHAR(32) NOT NULL DEFAULT '',
			size BIGINT NOT NULL DEFAULT 0,
			checksum VARCHAR(64) NOT NULL DEFAULT '',
			user_id UUID,
			created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP(0) WITH TIME ZONE
		)`,
		MySQL: `CREATE TABLE IF NOT EXISTS media (
			id CHAR(36) PRIMARY KEY,
			name VARCHAR(512) NOT NULL,
			storage_key VARCHAR(1024) NOT NULL,
			storage_driver VARCHAR(64) NOT NULL,
			mime_type VARCHAR(255) NOT NULL,
			kind VARCHAR(32) NOT NULL,
			extension VARCHAR(32) NOT NULL DEFAULT '',
			size BIGINT NOT NULL DEFAULT 0,
			checksum VARCHAR(64) NOT NULL DEFAULT '',
			user_id CHAR(36),
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NULL,
			INDEX idx_media_kind (kind),
			INDEX idx_media_user (user_id),
			INDEX idx_media_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		SQLite: `CREATE TABLE IF NOT EXISTS media (
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
		)`,
	}); err != nil {
		return err
	}

	if db.DriverName() == "mysql" {
		return nil
	}
	if err := migrations.CreateIndex(ctx, db, "idx_media_kind", "media", "kind"); err != nil {
		return err
	}
	if err := migrations.CreateIndex(ctx, db, "idx_media_user", "media", "user_id"); err != nil {
		return err
	}
	return migrations.CreateIndex(ctx, db, "idx_media_created", "media", "created_at")
}

func downMedia(ctx context.Context, db database.Database) error {
	if db.DriverName() != "mysql" {
		_ = migrations.DropIndex(ctx, db, "idx_media_kind", "media")
		_ = migrations.DropIndex(ctx, db, "idx_media_user", "media")
		_ = migrations.DropIndex(ctx, db, "idx_media_created", "media")
	}
	return migrations.DropTableIfExists(ctx, db, "media")
}

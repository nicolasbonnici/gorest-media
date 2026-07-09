package migrations

import (
	"context"
	"testing"

	"github.com/nicolasbonnici/gorest/database"

	_ "github.com/nicolasbonnici/gorest/database/sqlite"
)

func TestMigrationsUpDown(t *testing.T) {
	db, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	migs, err := GetMigrations().Migrations()
	if err != nil {
		t.Fatalf("Migrations: %v", err)
	}
	if len(migs) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(migs))
	}

	for _, m := range migs {
		if err := m.ExecuteUp(ctx, db); err != nil {
			t.Fatalf("up %s: %v", m.FullName(), err)
		}
	}

	if _, err := db.Exec(ctx, "INSERT INTO media (id, name, storage_key, storage_driver, mime_type, kind) VALUES ('1','n','k','local','image/png','image')"); err != nil {
		t.Fatalf("insert into created table: %v", err)
	}

	for i := len(migs) - 1; i >= 0; i-- {
		if err := migs[i].ExecuteDown(ctx, db); err != nil {
			t.Fatalf("down %s: %v", migs[i].FullName(), err)
		}
	}

	if _, err := db.Exec(ctx, "INSERT INTO media (id) VALUES ('1')"); err == nil {
		t.Error("expected media table to be gone after down migration")
	}
}

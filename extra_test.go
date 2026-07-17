package migrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDirFromDisk(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0001_create_users.up.sql":   "CREATE TABLE users (id BIGINT)",
		"0001_create_users.down.sql": "DROP TABLE users",
		"0002_create_posts.up.sql":   "CREATE TABLE posts (id BIGINT)",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	migs, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("load dir: %v", err)
	}
	if len(migs) != 2 || migs[0].Version != 1 || migs[1].Version != 2 {
		t.Fatalf("unexpected migrations: %+v", migs)
	}
}

func TestMigrationsReturnsSortedCopy(t *testing.T) {
	mg := New(nil)
	_ = mg.Register(
		Migration{Version: 2, Name: "b", UpSQL: "SELECT 1"},
		Migration{Version: 1, Name: "a", UpSQL: "SELECT 1"},
	)
	got := mg.Migrations()
	if len(got) != 2 || got[0].Version != 1 || got[1].Version != 2 {
		t.Fatalf("Migrations() = %+v, want sorted [1 2]", got)
	}
	// Mutating the copy must not affect the migrator.
	got[0].Name = "mutated"
	if mg.Migrations()[0].Name != "a" {
		t.Fatal("Migrations() returned a shared slice, not a copy")
	}
}

func TestRedoWithNothingApplied(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db)
	_ = mg.Register(Migration{Version: 1, Name: "a", UpSQL: "SELECT 1", DownSQL: "SELECT 1"})
	if err := mg.Redo(context.Background()); err != nil {
		t.Fatalf("redo on empty: %v", err)
	}
	if got := appliedVersions(t, mg); len(got) != 0 {
		t.Fatalf("applied = %v, want none", got)
	}
}

func TestDownSQLReversible(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db)
	_ = mg.Register(Migration{
		Version: 1,
		Name:    "sql",
		UpSQL:   "CREATE TABLE things (id BIGINT)",
		DownSQL: "DROP TABLE things",
	})
	ctx := context.Background()
	if err := mg.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := mg.Down(ctx, 1); err != nil {
		t.Fatalf("down: %v", err)
	}
	if got := appliedVersions(t, mg); len(got) != 0 {
		t.Fatalf("applied = %v, want none after down", got)
	}
}

func TestIrreversibleMigrationCannotRollBack(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db)
	// No Down / DownSQL => irreversible.
	_ = mg.Register(Migration{Version: 1, Name: "oneway", UpSQL: "CREATE TABLE x (id BIGINT)"})
	ctx := context.Background()
	if err := mg.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := mg.Down(ctx, 1); !errors.Is(err, ErrMissingMigration) {
		t.Fatalf("down err = %v, want ErrMissingMigration", err)
	}
}

func TestRollbackUnknownAppliedVersion(t *testing.T) {
	db, _ := openMem(t)
	ctx := context.Background()

	// First migrator applies version 1.
	mg1 := New(db)
	_ = mg1.Register(Migration{Version: 1, Name: "a", UpSQL: "SELECT 1", DownSQL: "SELECT 1"})
	if err := mg1.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Second migrator does not know about version 1, so it cannot roll back.
	mg2 := New(db)
	_ = mg2.Register(Migration{Version: 2, Name: "b", UpSQL: "SELECT 1", DownSQL: "SELECT 1"})
	if err := mg2.Down(ctx, 1); !errors.Is(err, ErrMissingMigration) {
		t.Fatalf("down err = %v, want ErrMissingMigration", err)
	}
}

func TestStatusReportsUnknownAppliedVersion(t *testing.T) {
	db, _ := openMem(t)
	ctx := context.Background()

	mg1 := New(db)
	_ = mg1.Register(Migration{Version: 1, Name: "a", UpSQL: "SELECT 1", DownSQL: "SELECT 1"})
	_ = mg1.Migrate(ctx)

	mg2 := New(db)
	_ = mg2.Register(Migration{Version: 2, Name: "b", UpSQL: "SELECT 1"})
	statuses, err := mg2.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("statuses = %+v, want 2 entries", statuses)
	}
	if statuses[0].Version != 1 || statuses[0].Name != "(unknown)" || !statuses[0].Applied {
		t.Fatalf("unexpected unknown entry: %+v", statuses[0])
	}
	if statuses[1].Version != 2 || statuses[1].Applied {
		t.Fatalf("unexpected known entry: %+v", statuses[1])
	}
}

func TestSchemaHelperTypes(t *testing.T) {
	got := CreateTable("all_types", func(t *Table) {
		t.Column("data", "JSON")
		t.Text("body", NotNull())
		t.Float("score")
		t.Timestamp("seen_at")
		t.Date("born_on")
		t.BigInteger("counter", PrimaryKey())
	}, WithoutID(), IfNotExists())

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS all_types (",
		"\tdata JSON,",
		"\tbody TEXT NOT NULL,",
		"\tscore DOUBLE PRECISION,",
		"\tseen_at TIMESTAMP,",
		"\tborn_on DATE,",
		"\tcounter BIGINT PRIMARY KEY",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestDefaultRawAndNumericDefault(t *testing.T) {
	got := CreateTable("t", func(t *Table) {
		t.Timestamp("created_at", NotNull(), DefaultRaw("CURRENT_TIMESTAMP"))
		t.Integer("hits", NotNull(), Default(0))
	}, WithoutID())
	if !strings.Contains(got, "created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP") {
		t.Fatalf("missing raw default:\n%s", got)
	}
	if !strings.Contains(got, "hits INTEGER NOT NULL DEFAULT 0") {
		t.Fatalf("missing numeric default:\n%s", got)
	}
}

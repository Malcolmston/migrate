package migrate

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"
)

func TestLoadFSOrdersAndPairs(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_add_email.up.sql":      {Data: []byte("ALTER TABLE users ADD COLUMN email VARCHAR(255)")},
		"0002_add_email.down.sql":    {Data: []byte("ALTER TABLE users DROP COLUMN email")},
		"0001_create_users.up.sql":   {Data: []byte("CREATE TABLE users (id BIGINT)")},
		"0001_create_users.down.sql": {Data: []byte("DROP TABLE users")},
		"10_create_posts.up.sql":     {Data: []byte("CREATE TABLE posts (id BIGINT)")},
		"README.md":                  {Data: []byte("ignored")},
		"notes.txt":                  {Data: []byte("ignored")},
	}
	migs, err := LoadFS(fsys)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(migs) != 3 {
		t.Fatalf("loaded %d migrations, want 3", len(migs))
	}
	wantVersions := []uint64{1, 2, 10}
	wantNames := []string{"create_users", "add_email", "create_posts"}
	for i, m := range migs {
		if m.Version != wantVersions[i] {
			t.Fatalf("migration %d version = %d, want %d", i, m.Version, wantVersions[i])
		}
		if m.Name != wantNames[i] {
			t.Fatalf("migration %d name = %q, want %q", i, m.Name, wantNames[i])
		}
	}
	// 0002 has both directions; 10 has only up.
	if migs[1].UpSQL == "" || migs[1].DownSQL == "" {
		t.Fatalf("version 2 should have up and down")
	}
	if migs[2].DownSQL != "" {
		t.Fatalf("version 10 should have no down")
	}
}

func TestLoadFSRunsThroughMigrator(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_create_users.up.sql":   {Data: []byte("CREATE TABLE users (id BIGINT)")},
		"0001_create_users.down.sql": {Data: []byte("DROP TABLE users")},
		"0002_create_posts.up.sql":   {Data: []byte("CREATE TABLE posts (id BIGINT)")},
		"0002_create_posts.down.sql": {Data: []byte("DROP TABLE posts")},
	}
	migs, err := LoadFS(fsys)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	db, _ := openMem(t)
	mg := New(db)
	if err := mg.Register(migs...); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := mg.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	statuses, err := mg.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(statuses) != 2 || !statuses[0].Applied || !statuses[1].Applied {
		t.Fatalf("status = %+v, want both applied", statuses)
	}
}

func TestLoadFSDuplicateUp(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_a.up.sql": {Data: []byte("SELECT 1")},
	}
	// A second up file for the same version is a duplicate; simulate by adding it.
	fsys["0001_a.up.sql"] = &fstest.MapFile{Data: []byte("SELECT 1")}
	// Add a genuinely conflicting name for the same version.
	fsys["0001_b.up.sql"] = &fstest.MapFile{Data: []byte("SELECT 1")}
	if _, err := LoadFS(fsys); err == nil {
		t.Fatal("expected error for mismatched names on same version")
	}
}

func TestLoadFSMissingUp(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_a.down.sql": {Data: []byte("DROP TABLE a")},
	}
	if _, err := LoadFS(fsys); !errors.Is(err, ErrInvalidMigration) {
		t.Fatalf("err = %v, want ErrInvalidMigration", err)
	}
}

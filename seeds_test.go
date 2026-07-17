package migrate

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestSeederIdempotent(t *testing.T) {
	db, dsn := openMem(t)
	ctx := context.Background()
	s := NewSeeder(db)

	runs := 0
	seed := func(ctx context.Context, tx *sql.Tx) error {
		runs++
		return Execute(ctx, tx, "INSERT INTO countries (name) VALUES (?)", "Norway")
	}

	if err := s.Run(ctx, "countries", seed); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := s.Run(ctx, "countries", seed); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if runs != 1 {
		t.Fatalf("seed body ran %d times, want 1", runs)
	}

	applied, err := s.Applied(ctx)
	if err != nil {
		t.Fatalf("applied: %v", err)
	}
	if _, ok := applied["countries"]; !ok {
		t.Fatalf("countries seed not recorded: %v", applied)
	}

	// The insert committed exactly once.
	inserts := 0
	for _, e := range memExecLog(dsn) {
		if strings.HasPrefix(e, "INSERT INTO countries") {
			inserts++
		}
	}
	if inserts != 1 {
		t.Fatalf("countries inserted %d times, want 1", inserts)
	}
}

func TestSeederRunSQL(t *testing.T) {
	db, _ := openMem(t)
	ctx := context.Background()
	s := NewSeeder(db, WithSeedTable("my_seeds"))
	if err := s.RunSQL(ctx, "roles",
		"INSERT INTO roles (name) VALUES ('admin')",
		"INSERT INTO roles (name) VALUES ('user')"); err != nil {
		t.Fatalf("run sql: %v", err)
	}
	applied, err := s.Applied(ctx)
	if err != nil {
		t.Fatalf("applied: %v", err)
	}
	if _, ok := applied["roles"]; !ok {
		t.Fatalf("roles seed not recorded")
	}
}

func TestSeederRollsBackOnError(t *testing.T) {
	db, dsn := openMem(t)
	ctx := context.Background()
	s := NewSeeder(db)

	err := s.Run(ctx, "boom", func(ctx context.Context, tx *sql.Tx) error {
		if e := Execute(ctx, tx, "INSERT INTO leaks (id) VALUES (1)"); e != nil {
			return e
		}
		return context.Canceled // force failure after a mutation
	})
	if err == nil {
		t.Fatal("expected error from failing seed")
	}
	// Nothing recorded, nothing leaked.
	applied, _ := s.Applied(ctx)
	if _, ok := applied["boom"]; ok {
		t.Fatal("failed seed should not be recorded")
	}
	for _, e := range memExecLog(dsn) {
		if strings.HasPrefix(e, "INSERT INTO leaks") {
			t.Fatalf("rolled-back insert leaked: %q", e)
		}
	}
}

func TestSeederInvalidTable(t *testing.T) {
	db, _ := openMem(t)
	s := NewSeeder(db, WithSeedTable("bad name"))
	if err := s.EnsureTable(context.Background()); err == nil {
		t.Fatal("expected invalid table name error")
	}
}

func TestExecuteAll(t *testing.T) {
	db, dsn := openMem(t)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := ExecuteAll(ctx, tx,
		"CREATE TABLE a (id BIGINT)",
		"",
		"CREATE TABLE b (id BIGINT)"); err != nil {
		t.Fatalf("execute all: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	assertContainsInOrder(t, memExecLog(dsn), "CREATE TABLE a", "CREATE TABLE b")
}

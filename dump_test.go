package migrate

import (
	"context"
	"strings"
	"testing"
)

func TestSchemaDump(t *testing.T) {
	d := NewSchemaDump(Postgres, 20240102)
	d.CreateTable("users", func(t *Table) {
		t.String("email", NotNull())
	}).
		AddIndex("users", []string{"email"}, UniqueIndex()).
		AddForeignKey("users", "accounts")

	out := d.String()
	for _, want := range []string{
		"-- dialect: postgres",
		"-- version: 20240102",
		`CREATE TABLE "users" (`,
		`CREATE UNIQUE INDEX "index_users_on_email" ON "users" ("email");`,
		`ALTER TABLE "users" ADD CONSTRAINT`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dump missing %q:\n%s", want, out)
		}
	}
	// Every statement is terminated with a semicolon-newline.
	if strings.Count(out, ";\n") != len(d.Statements()) {
		t.Fatalf("expected %d terminated statements:\n%s", len(d.Statements()), out)
	}
	if d.Version() != 20240102 {
		t.Fatalf("version = %d", d.Version())
	}
}

func TestSchemaDumpSetVersionAndAdd(t *testing.T) {
	d := NewSchemaDump(nil, 0).SetVersion(7).Add("CREATE TABLE x (id BIGINT)")
	if d.Version() != 7 {
		t.Fatalf("version = %d, want 7", d.Version())
	}
	if got := d.Statements(); len(got) != 1 || got[0] != "CREATE TABLE x (id BIGINT)" {
		t.Fatalf("statements = %v", got)
	}
	if !strings.Contains(d.String(), "-- dialect: ansi") {
		t.Fatal("nil dialect should default to ansi")
	}
}

func TestMigratorVersion(t *testing.T) {
	db, _ := openMem(t)
	ctx := context.Background()
	mg := New(db)

	v, err := mg.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if v != 0 {
		t.Fatalf("empty version = %d, want 0", v)
	}

	var order []uint64
	_ = mg.Register(tracking(10, "a", &order), tracking(25, "b", &order))
	if err := mg.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	v, err = mg.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if v != 25 {
		t.Fatalf("version = %d, want 25", v)
	}
}

package migrate

import (
	"strings"
	"testing"
)

func TestSchemaDumpExtras(t *testing.T) {
	d := NewSchemaDump(ANSI, 42).
		EnableExtension("pgcrypto").
		CreateEnum("mood", []string{"sad", "happy"}).
		CreateJoinTable("assemblies", "parts", nil).
		AddUniqueConstraint("users", []string{"email"}).
		AddCheckConstraint("products", "price > 0").
		CreateView("active_users", "SELECT * FROM users WHERE active")

	stmts := d.Statements()
	want := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		"CREATE TYPE mood AS ENUM ('sad', 'happy')",
		"CREATE TABLE assemblies_parts (\n\tassemblie_id BIGINT NOT NULL,\n\tpart_id BIGINT NOT NULL\n)",
		"ALTER TABLE users ADD CONSTRAINT uniq_users_email UNIQUE (email)",
		"ALTER TABLE products ADD CONSTRAINT chk_products_price___0 CHECK (price > 0)",
		"CREATE VIEW active_users AS SELECT * FROM users WHERE active",
	}
	if len(stmts) != len(want) {
		t.Fatalf("got %d statements, want %d: %v", len(stmts), len(want), stmts)
	}
	for i := range want {
		if stmts[i] != want[i] {
			t.Errorf("statement %d:\ngot  %q\nwant %q", i, stmts[i], want[i])
		}
	}

	out := d.String()
	if !strings.Contains(out, "-- version: 42") {
		t.Errorf("dump missing version header:\n%s", out)
	}
	if !strings.Contains(out, "CREATE VIEW active_users AS SELECT * FROM users WHERE active;") {
		t.Errorf("dump missing view statement:\n%s", out)
	}
}

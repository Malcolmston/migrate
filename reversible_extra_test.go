package migrate

import (
	"context"
	"testing"
)

func TestChangeExtrasReversibleRoundTrip(t *testing.T) {
	db, dsn := openMem(t)
	ctx := context.Background()
	mg := New(db)

	m := Change(1, "extras", func(r *ChangeRecorder) {
		r.EnableExtension("pgcrypto")
		r.CreateView("active_users", "SELECT * FROM users WHERE active")
		r.AddCheckConstraint("products", "price > 0")
		r.RenameIndex("products", "old_idx", "new_idx")
	})
	if err := mg.Register(m); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := mg.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	assertContainsInOrder(t, memExecLog(dsn),
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		"CREATE VIEW active_users AS SELECT * FROM users WHERE active",
		"ALTER TABLE products ADD CONSTRAINT chk_products_price___0 CHECK (price > 0)",
		"ALTER INDEX old_idx RENAME TO new_idx",
	)

	if err := mg.Down(ctx, 1); err != nil {
		t.Fatalf("down: %v", err)
	}
	// Inverse runs in reverse record order.
	assertContainsInOrder(t, memExecLog(dsn),
		"ALTER INDEX new_idx RENAME TO old_idx",
		"ALTER TABLE products DROP CONSTRAINT chk_products_price___0",
		"DROP VIEW active_users",
		`DROP EXTENSION IF EXISTS "pgcrypto"`,
	)
}

func TestChangeJoinTableReversible(t *testing.T) {
	r := &ChangeRecorder{schema: NewSchema(ANSI)}
	r.CreateJoinTable("assemblies", "parts", nil)
	r.AddUniqueConstraint("assemblies_parts", []string{"assemblie_id", "part_id"})
	if !r.Reversible() {
		t.Fatal("expected reversible change")
	}
	ups := r.upStatements()
	if len(ups) != 2 {
		t.Fatalf("want 2 up statements, got %d: %v", len(ups), ups)
	}
	downs, err := r.downStatements()
	if err != nil {
		t.Fatalf("downStatements: %v", err)
	}
	if downs[0] != "ALTER TABLE assemblies_parts DROP CONSTRAINT uniq_assemblies_parts_assemblie_id_part_id" {
		t.Errorf("first down = %q", downs[0])
	}
	if downs[1] != "DROP TABLE assemblies_parts" {
		t.Errorf("second down = %q", downs[1])
	}
}

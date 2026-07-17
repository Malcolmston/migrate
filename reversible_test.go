package migrate

import (
	"context"
	"errors"
	"testing"
)

func TestChangeReversibleRoundTrip(t *testing.T) {
	db, dsn := openMem(t)
	ctx := context.Background()
	mg := New(db)

	m := Change(1, "create_users", func(r *ChangeRecorder) {
		r.CreateTable("users", func(t *Table) {
			t.String("email", NotNull())
		})
		r.AddColumn("users", "nickname", "VARCHAR", Limit(50))
		r.AddIndex("users", []string{"email"}, UniqueIndex())
	})
	if err := mg.Register(m); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := mg.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Forward ran create, add column, add index (in order).
	log := memExecLog(dsn)
	assertContainsInOrder(t, log,
		"CREATE TABLE users",
		"ALTER TABLE users ADD COLUMN nickname VARCHAR(50)",
		"CREATE UNIQUE INDEX index_users_on_email ON users (email)",
	)

	// Rolling back runs the inverse in reverse order: drop index, drop column,
	// drop table.
	if err := mg.Down(ctx, 1); err != nil {
		t.Fatalf("down: %v", err)
	}
	log = memExecLog(dsn)
	assertContainsInOrder(t, log,
		"DROP INDEX index_users_on_email",
		"ALTER TABLE users DROP COLUMN nickname",
		"DROP TABLE users",
	)
	if got := appliedVersions(t, mg); len(got) != 0 {
		t.Fatalf("applied = %v, want none after down", got)
	}
}

func TestChangeIrreversibleDetected(t *testing.T) {
	db, _ := openMem(t)
	ctx := context.Background()
	mg := New(db)

	m := Change(1, "risky", func(r *ChangeRecorder) {
		r.CreateTable("t", func(tb *Table) { tb.Integer("n") })
		r.Execute("UPDATE t SET n = n + 1")
	})
	if err := mg.Register(m); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := mg.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// The raw Execute makes the change irreversible.
	if err := mg.Down(ctx, 1); !errors.Is(err, ErrIrreversibleMigration) {
		t.Fatalf("down err = %v, want ErrIrreversibleMigration", err)
	}
}

func TestChangeRecorderInverseRules(t *testing.T) {
	// remove_column without a type is irreversible; with a type it is not.
	r1 := &ChangeRecorder{schema: ansiSchema}
	r1.RemoveColumn("users", "legacy", "")
	if r1.Reversible() {
		t.Fatal("remove_column without type should be irreversible")
	}
	r2 := &ChangeRecorder{schema: ansiSchema}
	r2.RemoveColumn("users", "legacy", "VARCHAR", Limit(20))
	if !r2.Reversible() {
		t.Fatal("remove_column with type should be reversible")
	}
	down, err := r2.downStatements()
	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if len(down) != 1 || down[0] != "ALTER TABLE users ADD COLUMN legacy VARCHAR(20)" {
		t.Fatalf("unexpected inverse: %v", down)
	}

	// drop_table without a rebuild block is irreversible.
	r3 := &ChangeRecorder{schema: ansiSchema}
	r3.DropTable("gone", nil)
	if r3.Reversible() {
		t.Fatal("drop_table without block should be irreversible")
	}
	// change_column is irreversible.
	r4 := &ChangeRecorder{schema: ansiSchema}
	r4.ChangeColumn("users", "age", "BIGINT")
	if r4.Reversible() {
		t.Fatal("change_column should be irreversible")
	}
}

func TestChangeRecorderRichInverse(t *testing.T) {
	r := &ChangeRecorder{schema: ansiSchema}
	r.RenameTable("a", "b")
	r.RenameColumn("b", "x", "y")
	r.AddReference("b", "owner", WithForeignKey())
	r.AddForeignKey("b", "accounts")
	r.AddTimestamps("b")
	down, err := r.downStatements()
	if err != nil {
		t.Fatalf("down: %v", err)
	}
	// Inverse is emitted in reverse record order.
	want := []string{
		"ALTER TABLE b DROP COLUMN updated_at",
		"ALTER TABLE b DROP COLUMN created_at",
		"ALTER TABLE b DROP CONSTRAINT fk_b_account_id",
		"ALTER TABLE b DROP COLUMN owner_id",
		"ALTER TABLE b RENAME COLUMN y TO x",
		"ALTER TABLE b RENAME TO a",
	}
	if len(down) != len(want) {
		t.Fatalf("down = %v, want %v", down, want)
	}
	for i := range want {
		if down[i] != want[i] {
			t.Fatalf("inverse %d:\ngot  %q\nwant %q", i, down[i], want[i])
		}
	}
}

func TestChangeWithDialect(t *testing.T) {
	m := ChangeWith(Postgres, 1, "create", func(r *ChangeRecorder) {
		r.CreateTable("t", func(tb *Table) { tb.String("s") })
	})
	rec := &ChangeRecorder{schema: NewSchema(Postgres)}
	rec.CreateTable("t", func(tb *Table) { tb.String("s") })
	if m.Version != 1 || m.Name != "create" {
		t.Fatalf("unexpected migration meta: %+v", m)
	}
	// The recorder renders postgres-quoted identifiers.
	up := rec.upStatements()
	if len(up) != 1 || up[0][:14] != `CREATE TABLE "` {
		t.Fatalf("expected quoted postgres DDL, got %q", up[0])
	}
}

// assertContainsInOrder asserts that each needle appears in log, and that they
// appear in the given relative order.
func assertContainsInOrder(t *testing.T, log []string, needles ...string) {
	t.Helper()
	idx := 0
	for _, entry := range log {
		if idx < len(needles) && startsWith(entry, needles[idx]) {
			idx++
		}
	}
	if idx != len(needles) {
		t.Fatalf("log did not contain expected statements in order (matched %d/%d):\n%v",
			idx, len(needles), log)
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

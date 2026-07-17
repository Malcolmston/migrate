package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

// openMem opens an isolated memdb database for a test and registers cleanup.
func openMem(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := t.Name()
	resetMemDB(dsn)
	db, err := sql.Open("memdb", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		resetMemDB(dsn)
	})
	return db, dsn
}

// tracking builds a Go migration whose Up/Down append the version to *order and
// run a DDL statement so we can observe it in the exec log.
func tracking(version uint64, name string, order *[]uint64) Migration {
	return Migration{
		Version: version,
		Name:    name,
		Up: func(ctx context.Context, tx *sql.Tx) error {
			*order = append(*order, version)
			_, err := tx.ExecContext(ctx, fmt.Sprintf("CREATE TABLE t_%d (id BIGINT)", version))
			return err
		},
		Down: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE t_%d", version))
			return err
		},
	}
}

func appliedVersions(t *testing.T, mg *Migrator) []uint64 {
	t.Helper()
	statuses, err := mg.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	var out []uint64
	for _, s := range statuses {
		if s.Applied {
			out = append(out, s.Version)
		}
	}
	return out
}

func equalU64(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMigrateAppliesInAscendingOrder(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db)
	var order []uint64
	// Register out of order to prove sorting.
	if err := mg.Register(tracking(3, "third", &order), tracking(1, "first", &order), tracking(2, "second", &order)); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := mg.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if want := []uint64{1, 2, 3}; !equalU64(order, want) {
		t.Fatalf("apply order = %v, want %v", order, want)
	}
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1, 2, 3}) {
		t.Fatalf("recorded versions = %v, want [1 2 3]", got)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db, dsn := openMem(t)
	mg := New(db)
	var order []uint64
	if err := mg.Register(tracking(1, "a", &order), tracking(2, "b", &order)); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := mg.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate 1: %v", err)
	}
	logLen := len(memExecLog(dsn))

	// Second run must apply nothing new.
	order = nil
	if err := mg.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate 2: %v", err)
	}
	if len(order) != 0 {
		t.Fatalf("second migrate applied %v, want nothing", order)
	}
	if got := len(memExecLog(dsn)); got != logLen {
		t.Fatalf("exec log grew from %d to %d on no-op migrate", logLen, got)
	}
}

func TestDownRollsBackLatest(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db)
	var order []uint64
	_ = mg.Register(tracking(1, "a", &order), tracking(2, "b", &order), tracking(3, "c", &order))
	if err := mg.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := mg.Down(context.Background(), 1); err != nil {
		t.Fatalf("down: %v", err)
	}
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1, 2}) {
		t.Fatalf("after down = %v, want [1 2]", got)
	}
}

func TestRollbackDefaultsToOneStep(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db)
	var order []uint64
	_ = mg.Register(tracking(1, "a", &order), tracking(2, "b", &order))
	_ = mg.Migrate(context.Background())
	if err := mg.Rollback(context.Background(), 0); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1}) {
		t.Fatalf("after rollback = %v, want [1]", got)
	}
}

func TestMigrateToBothDirections(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db)
	var order []uint64
	_ = mg.Register(
		tracking(1, "a", &order),
		tracking(2, "b", &order),
		tracking(3, "c", &order),
		tracking(4, "d", &order),
	)
	ctx := context.Background()

	// Forward to 3.
	if err := mg.MigrateTo(ctx, 3); err != nil {
		t.Fatalf("migrate to 3: %v", err)
	}
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1, 2, 3}) {
		t.Fatalf("after MigrateTo(3) = %v, want [1 2 3]", got)
	}

	// Backward to 1.
	if err := mg.MigrateTo(ctx, 1); err != nil {
		t.Fatalf("migrate to 1: %v", err)
	}
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1}) {
		t.Fatalf("after MigrateTo(1) = %v, want [1]", got)
	}

	// All the way up.
	if err := mg.MigrateTo(ctx, 4); err != nil {
		t.Fatalf("migrate to 4: %v", err)
	}
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1, 2, 3, 4}) {
		t.Fatalf("after MigrateTo(4) = %v, want [1 2 3 4]", got)
	}

	// All the way down.
	if err := mg.MigrateTo(ctx, 0); err != nil {
		t.Fatalf("migrate to 0: %v", err)
	}
	if got := appliedVersions(t, mg); len(got) != 0 {
		t.Fatalf("after MigrateTo(0) = %v, want none", got)
	}
}

func TestUpN(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db)
	var order []uint64
	_ = mg.Register(tracking(1, "a", &order), tracking(2, "b", &order), tracking(3, "c", &order))
	if err := mg.Up(context.Background(), 2); err != nil {
		t.Fatalf("up 2: %v", err)
	}
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1, 2}) {
		t.Fatalf("after Up(2) = %v, want [1 2]", got)
	}
}

func TestRedo(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db)
	var order []uint64
	_ = mg.Register(tracking(1, "a", &order), tracking(2, "b", &order))
	_ = mg.Migrate(context.Background())

	order = nil
	if err := mg.Redo(context.Background()); err != nil {
		t.Fatalf("redo: %v", err)
	}
	// Redo re-applies exactly the latest version (2), not 1.
	if !equalU64(order, []uint64{2}) {
		t.Fatalf("redo re-applied %v, want [2]", order)
	}
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1, 2}) {
		t.Fatalf("after redo = %v, want [1 2]", got)
	}
}

// failingMigration errors from its Up body after running a DDL statement, to
// prove the transaction rolls back and nothing is recorded.
func failingMigration(version uint64) Migration {
	return Migration{
		Version: version,
		Name:    "boom",
		Up: func(ctx context.Context, tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("CREATE TABLE leaked_%d (id BIGINT)", version)); err != nil {
				return err
			}
			return errors.New("intentional failure")
		},
	}
}

func TestFailingMigrationRollsBackAndHalts(t *testing.T) {
	db, dsn := openMem(t)
	mg := New(db)
	var order []uint64
	_ = mg.Register(
		tracking(1, "a", &order),
		failingMigration(2),
		tracking(3, "c", &order),
	)

	err := mg.Migrate(context.Background())
	if err == nil {
		t.Fatal("expected error from failing migration")
	}

	// Only version 1 applied; the run halted at 2 and never reached 3.
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1}) {
		t.Fatalf("applied = %v, want [1]", got)
	}
	if !equalU64(order, []uint64{1}) {
		t.Fatalf("ran up bodies %v, want [1]", order)
	}

	// The DDL from the failed migration must not have been committed.
	for _, entry := range memExecLog(dsn) {
		if entry == "CREATE TABLE leaked_2 (id BIGINT)" {
			t.Fatalf("rolled-back DDL leaked into exec log: %q", entry)
		}
	}
}

func TestDuplicateVersionRejected(t *testing.T) {
	mg := New(nil)
	err := mg.Register(
		Migration{Version: 1, Name: "a", UpSQL: "SELECT 1"},
		Migration{Version: 1, Name: "b", UpSQL: "SELECT 1"},
	)
	if !errors.Is(err, ErrDuplicateVersion) {
		t.Fatalf("err = %v, want ErrDuplicateVersion", err)
	}
}

func TestInvalidMigrationRejected(t *testing.T) {
	mg := New(nil)
	if err := mg.Register(Migration{Version: 0, Name: "x", UpSQL: "SELECT 1"}); !errors.Is(err, ErrInvalidMigration) {
		t.Fatalf("zero version err = %v, want ErrInvalidMigration", err)
	}
	if err := mg.Register(Migration{Version: 5, Name: "x"}); !errors.Is(err, ErrInvalidMigration) {
		t.Fatalf("no-up err = %v, want ErrInvalidMigration", err)
	}
}

func TestInvalidTableName(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db, WithTable("bad name; DROP TABLE x"))
	if err := mg.Migrate(context.Background()); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("err = %v, want ErrInvalidTableName", err)
	}
}

func TestCustomTableName(t *testing.T) {
	db, _ := openMem(t)
	mg := New(db, WithTable("my_versions"))
	var order []uint64
	_ = mg.Register(tracking(1, "a", &order))
	if err := mg.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := appliedVersions(t, mg); !equalU64(got, []uint64{1}) {
		t.Fatalf("applied = %v, want [1]", got)
	}
}

func TestSQLTextMigration(t *testing.T) {
	db, dsn := openMem(t)
	mg := New(db)
	_ = mg.Register(Migration{
		Version: 1,
		Name:    "sql_up",
		UpSQL:   "CREATE TABLE widgets (id BIGINT); CREATE TABLE gadgets (id BIGINT)",
		DownSQL: "DROP TABLE gadgets; DROP TABLE widgets",
	})
	if err := mg.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	log := memExecLog(dsn)
	var sawWidgets, sawGadgets bool
	for _, e := range log {
		if e == "CREATE TABLE widgets (id BIGINT)" {
			sawWidgets = true
		}
		if e == "CREATE TABLE gadgets (id BIGINT)" {
			sawGadgets = true
		}
	}
	if !sawWidgets || !sawGadgets {
		t.Fatalf("multi-statement SQL not fully executed: %v", log)
	}
}

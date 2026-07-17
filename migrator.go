package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"time"
)

// DefaultTable is the name of the bookkeeping table used when no other name is
// configured via [WithTable].
const DefaultTable = "schema_migrations"

// identRe matches a safe, unqualified SQL identifier. The table name is
// interpolated directly into statements, so it must be validated.
var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Migrator applies and reverses [Migration]s against a *sql.DB, tracking which
// versions have been applied in a bookkeeping table.
//
// A Migrator is safe to construct with [New] and configure with [Option]s. Its
// methods are not safe for concurrent use from multiple goroutines against the
// same database; run migrations from a single goroutine.
type Migrator struct {
	db         *sql.DB
	table      string
	migrations []Migration // kept sorted ascending by Version
}

// Option configures a [Migrator].
type Option func(*Migrator)

// WithTable overrides the bookkeeping table name (default [DefaultTable]).
func WithTable(name string) Option {
	return func(m *Migrator) { m.table = name }
}

// New creates a Migrator over db. It panics only on a programming error such as
// a nil db; configuration problems (like an invalid table name) surface from
// the first method call. Use [Migrator.Validate] to check eagerly.
func New(db *sql.DB, opts ...Option) *Migrator {
	m := &Migrator{db: db, table: DefaultTable}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Validate reports whether the migrator is safely configured.
func (mg *Migrator) Validate() error {
	if !identRe.MatchString(mg.table) {
		return fmt.Errorf("%w: %q", ErrInvalidTableName, mg.table)
	}
	return nil
}

// Register adds migrations, keeping the internal set sorted by Version and
// rejecting duplicate versions or malformed migrations.
func (mg *Migrator) Register(ms ...Migration) error {
	seen := make(map[uint64]struct{}, len(mg.migrations))
	for _, existing := range mg.migrations {
		seen[existing.Version] = struct{}{}
	}
	for _, m := range ms {
		if err := m.validate(); err != nil {
			return err
		}
		if _, dup := seen[m.Version]; dup {
			return fmt.Errorf("%w: %d", ErrDuplicateVersion, m.Version)
		}
		seen[m.Version] = struct{}{}
		mg.migrations = append(mg.migrations, m)
	}
	sort.Slice(mg.migrations, func(i, j int) bool {
		return mg.migrations[i].Version < mg.migrations[j].Version
	})
	return nil
}

// Migrations returns a copy of the registered migrations in ascending order.
func (mg *Migrator) Migrations() []Migration {
	out := make([]Migration, len(mg.migrations))
	copy(out, mg.migrations)
	return out
}

// EnsureSchemaTable creates the bookkeeping table if it does not already exist.
// It is called implicitly by the other methods but is exported for callers that
// want to provision the table up front.
func (mg *Migrator) EnsureSchemaTable(ctx context.Context) error {
	if err := mg.Validate(); err != nil {
		return err
	}
	stmt := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (\n"+
			"\tversion BIGINT NOT NULL PRIMARY KEY,\n"+
			"\tapplied_at TIMESTAMP NOT NULL\n"+
			")", mg.table)
	if _, err := mg.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("ensure schema table: %w", err)
	}
	return nil
}

// appliedSet returns the set of applied versions and the time each was applied.
func (mg *Migrator) appliedSet(ctx context.Context) (map[uint64]time.Time, error) {
	rows, err := mg.db.QueryContext(ctx,
		fmt.Sprintf("SELECT version, applied_at FROM %s ORDER BY version ASC", mg.table))
	if err != nil {
		return nil, fmt.Errorf("query applied versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[uint64]time.Time)
	for rows.Next() {
		var (
			version uint64
			at      time.Time
		)
		if err := rows.Scan(&version, &at); err != nil {
			return nil, fmt.Errorf("scan applied version: %w", err)
		}
		applied[version] = at
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied versions: %w", err)
	}
	return applied, nil
}

// runUp applies a single migration inside its own transaction and records it.
func (mg *Migrator) runUp(ctx context.Context, m Migration) error {
	tx, err := mg.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for up %d: %w", m.Version, err)
	}
	if err := m.applyUp(ctx, tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply up %d %q: %w", m.Version, m.Name, err)
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (version, applied_at) VALUES (?, ?)", mg.table),
		int64(m.Version), time.Now().UTC()); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record version %d: %w", m.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit up %d: %w", m.Version, err)
	}
	return nil
}

// runDown reverses a single migration inside its own transaction and removes
// its bookkeeping row.
func (mg *Migrator) runDown(ctx context.Context, m Migration) error {
	if !m.reversible() {
		return fmt.Errorf("%w: version %d %q", ErrMissingMigration, m.Version, m.Name)
	}
	tx, err := mg.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for down %d: %w", m.Version, err)
	}
	if err := m.applyDown(ctx, tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply down %d %q: %w", m.Version, m.Name, err)
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE version = ?", mg.table),
		int64(m.Version)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("remove version %d: %w", m.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit down %d: %w", m.Version, err)
	}
	return nil
}

// pending returns registered migrations that have not been applied, ascending.
func (mg *Migrator) pending(applied map[uint64]time.Time) []Migration {
	var out []Migration
	for _, m := range mg.migrations {
		if _, ok := applied[m.Version]; !ok {
			out = append(out, m)
		}
	}
	return out
}

// appliedDesc returns registered migrations that have been applied, descending.
// Applied versions with no registered migration are reported via the error.
func (mg *Migrator) appliedDesc(applied map[uint64]time.Time) ([]Migration, error) {
	known := make(map[uint64]struct{}, len(mg.migrations))
	var out []Migration
	for _, m := range mg.migrations {
		known[m.Version] = struct{}{}
		if _, ok := applied[m.Version]; ok {
			out = append(out, m)
		}
	}
	for v := range applied {
		if _, ok := known[v]; !ok {
			return nil, fmt.Errorf("%w: %d", ErrMissingMigration, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version > out[j].Version })
	return out, nil
}

// Migrate applies every pending migration in ascending order. It is idempotent:
// migrations already recorded are skipped. A failing migration halts the run.
func (mg *Migrator) Migrate(ctx context.Context) error {
	if err := mg.EnsureSchemaTable(ctx); err != nil {
		return err
	}
	applied, err := mg.appliedSet(ctx)
	if err != nil {
		return err
	}
	for _, m := range mg.pending(applied) {
		if err := mg.runUp(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// Up applies the next n pending migrations in ascending order. If n <= 0 all
// pending migrations are applied (equivalent to [Migrator.Migrate]).
func (mg *Migrator) Up(ctx context.Context, n int) error {
	if err := mg.EnsureSchemaTable(ctx); err != nil {
		return err
	}
	applied, err := mg.appliedSet(ctx)
	if err != nil {
		return err
	}
	pending := mg.pending(applied)
	if n > 0 && n < len(pending) {
		pending = pending[:n]
	}
	for _, m := range pending {
		if err := mg.runUp(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// Down rolls back the n most recently applied migrations in descending order.
// If n <= 0 every applied migration is rolled back.
func (mg *Migrator) Down(ctx context.Context, n int) error {
	if err := mg.EnsureSchemaTable(ctx); err != nil {
		return err
	}
	applied, err := mg.appliedSet(ctx)
	if err != nil {
		return err
	}
	toRollback, err := mg.appliedDesc(applied)
	if err != nil {
		return err
	}
	if n > 0 && n < len(toRollback) {
		toRollback = toRollback[:n]
	}
	for _, m := range toRollback {
		if err := mg.runDown(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// Rollback is a convenience alias for rolling back the given number of steps.
// Rollback(ctx, 1) reverses only the most recent migration.
func (mg *Migrator) Rollback(ctx context.Context, steps int) error {
	if steps <= 0 {
		steps = 1
	}
	return mg.Down(ctx, steps)
}

// MigrateTo moves the schema to exactly the given target version. Migrations
// above the target that are applied are rolled back (descending); pending
// migrations at or below the target are applied (ascending). A target of 0
// rolls everything back.
func (mg *Migrator) MigrateTo(ctx context.Context, target uint64) error {
	if err := mg.EnsureSchemaTable(ctx); err != nil {
		return err
	}
	applied, err := mg.appliedSet(ctx)
	if err != nil {
		return err
	}

	downs, err := mg.appliedDesc(applied)
	if err != nil {
		return err
	}
	for _, m := range downs {
		if m.Version > target {
			if err := mg.runDown(ctx, m); err != nil {
				return err
			}
		}
	}
	for _, m := range mg.pending(applied) {
		if m.Version <= target {
			if err := mg.runUp(ctx, m); err != nil {
				return err
			}
		}
	}
	return nil
}

// Redo rolls back the most recently applied migration and re-applies it. It is
// a no-op when nothing has been applied.
func (mg *Migrator) Redo(ctx context.Context) error {
	if err := mg.EnsureSchemaTable(ctx); err != nil {
		return err
	}
	applied, err := mg.appliedSet(ctx)
	if err != nil {
		return err
	}
	desc, err := mg.appliedDesc(applied)
	if err != nil {
		return err
	}
	if len(desc) == 0 {
		return nil
	}
	latest := desc[0]
	if err := mg.runDown(ctx, latest); err != nil {
		return err
	}
	return mg.runUp(ctx, latest)
}

// MigrationStatus describes whether a known migration has been applied.
type MigrationStatus struct {
	Version   uint64
	Name      string
	Applied   bool
	AppliedAt time.Time // zero when not applied
}

// Status returns the state of every registered migration, plus any applied
// version that has no registered migration (reported with Name "(unknown)"),
// sorted ascending by version.
func (mg *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := mg.EnsureSchemaTable(ctx); err != nil {
		return nil, err
	}
	applied, err := mg.appliedSet(ctx)
	if err != nil {
		return nil, err
	}

	known := make(map[uint64]struct{}, len(mg.migrations))
	out := make([]MigrationStatus, 0, len(mg.migrations))
	for _, m := range mg.migrations {
		known[m.Version] = struct{}{}
		at, ok := applied[m.Version]
		out = append(out, MigrationStatus{
			Version:   m.Version,
			Name:      m.Name,
			Applied:   ok,
			AppliedAt: at,
		})
	}
	for v, at := range applied {
		if _, ok := known[v]; !ok {
			out = append(out, MigrationStatus{Version: v, Name: "(unknown)", Applied: true, AppliedAt: at})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

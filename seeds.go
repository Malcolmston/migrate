package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DefaultSeedTable is the bookkeeping table used to track applied seeds when no
// other name is configured.
const DefaultSeedTable = "schema_seeds"

// Seeder loads data into the database idempotently. Each seed is identified by a
// unique name; once a seed has run successfully its name is recorded, and
// subsequent runs skip it. This makes seed loading safe to invoke on every boot,
// mirroring the intent of Rails' db/seeds with added tracking.
//
// A Seeder is not safe for concurrent use against the same database; run seeds
// from a single goroutine.
type Seeder struct {
	db    *sql.DB
	table string
}

// SeedOption configures a [Seeder].
type SeedOption func(*Seeder)

// WithSeedTable overrides the seed bookkeeping table name (default
// [DefaultSeedTable]).
func WithSeedTable(name string) SeedOption {
	return func(s *Seeder) { s.table = name }
}

// NewSeeder creates a Seeder over db.
func NewSeeder(db *sql.DB, opts ...SeedOption) *Seeder {
	s := &Seeder{db: db, table: DefaultSeedTable}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// validate reports whether the seed table name is a safe identifier.
func (s *Seeder) validate() error {
	if !identRe.MatchString(s.table) {
		return fmt.Errorf("%w: %q", ErrInvalidTableName, s.table)
	}
	return nil
}

// EnsureTable creates the seed bookkeeping table if it does not already exist.
func (s *Seeder) EnsureTable(ctx context.Context) error {
	if err := s.validate(); err != nil {
		return err
	}
	stmt := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (\n"+
			"\tname VARCHAR(255) NOT NULL PRIMARY KEY,\n"+
			"\tapplied_at TIMESTAMP NOT NULL\n"+
			")", s.table)
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("ensure seed table: %w", err)
	}
	return nil
}

// Applied returns the set of seed names that have already run.
func (s *Seeder) Applied(ctx context.Context) (map[string]time.Time, error) {
	if err := s.EnsureTable(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf("SELECT name, applied_at FROM %s ORDER BY name ASC", s.table))
	if err != nil {
		return nil, fmt.Errorf("query applied seeds: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]time.Time)
	for rows.Next() {
		var (
			name string
			at   time.Time
		)
		if err := rows.Scan(&name, &at); err != nil {
			return nil, fmt.Errorf("scan seed: %w", err)
		}
		out[name] = at
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate seeds: %w", err)
	}
	return out, nil
}

// Run executes the named seed inside its own transaction and records it, but
// only if it has not already run. If the seed has run before, Run is a no-op and
// returns nil. The fn callback receives the transaction and must not commit or
// roll it back; the Seeder owns the transaction lifecycle. A failing fn rolls
// the transaction back and the seed is not recorded.
func (s *Seeder) Run(ctx context.Context, name string, fn func(ctx context.Context, tx *sql.Tx) error) error {
	if err := s.EnsureTable(ctx); err != nil {
		return err
	}
	applied, err := s.Applied(ctx)
	if err != nil {
		return err
	}
	if _, ok := applied[name]; ok {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed %q: %w", name, err)
	}
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("run seed %q: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (name, applied_at) VALUES (?, ?)", s.table),
		name, time.Now().UTC()); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record seed %q: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed %q: %w", name, err)
	}
	return nil
}

// RunSQL is a convenience wrapper around [Seeder.Run] that executes one or more
// raw SQL statements as the seed body.
func (s *Seeder) RunSQL(ctx context.Context, name string, statements ...string) error {
	return s.Run(ctx, name, func(ctx context.Context, tx *sql.Tx) error {
		return execStatements(ctx, tx, statements)
	})
}

// Execute runs a single raw SQL statement inside tx. It is a small convenience
// for Go migrations and seeds that want to run one statement without threading
// the result value.
func Execute(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("execute %q: %w", truncate(query, 60), err)
	}
	return nil
}

// ExecuteAll runs several raw SQL statements inside tx in order, stopping at the
// first error.
func ExecuteAll(ctx context.Context, tx *sql.Tx, statements ...string) error {
	return execStatements(ctx, tx, statements)
}

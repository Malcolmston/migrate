package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MigrateFunc is the signature of a Go migration direction. It receives the
// per-migration transaction and must not commit or roll it back; the [Migrator]
// owns the transaction lifecycle.
type MigrateFunc func(ctx context.Context, tx *sql.Tx) error

// Migration describes a single, reversible change to the database schema.
//
// A migration is identified by its Version, which must be unique within a
// [Migrator] and is used to order migrations. The direction to apply may be
// given either as a Go function (Up / Down) or as SQL text (UpSQL / DownSQL).
// When a Go function is present it takes precedence over the SQL text for that
// direction.
//
// SQL text may contain multiple statements separated by semicolons; each
// non-empty statement is executed in order within the migration's transaction.
type Migration struct {
	// Version orders the migration and uniquely identifies it. A common
	// convention is a UTC timestamp such as 20240117153000.
	Version uint64

	// Name is a short human readable label, e.g. "create_users".
	Name string

	// Up applies the migration. Optional if UpSQL is set.
	Up MigrateFunc
	// Down reverses the migration. Optional; a migration with neither Down nor
	// DownSQL is irreversible and cannot be rolled back.
	Down MigrateFunc

	// UpSQL / DownSQL provide the directions as raw SQL. Ignored for a
	// direction whose Go function is non-nil.
	UpSQL   string
	DownSQL string
}

// validate reports whether the migration is well formed for registration.
func (m Migration) validate() error {
	if m.Version == 0 {
		return fmt.Errorf("%w: version must be non-zero (name %q)", ErrInvalidMigration, m.Name)
	}
	if m.Up == nil && strings.TrimSpace(m.UpSQL) == "" {
		return fmt.Errorf("%w: version %d %q has no Up direction", ErrInvalidMigration, m.Version, m.Name)
	}
	return nil
}

// reversible reports whether a Down direction is available.
func (m Migration) reversible() bool {
	return m.Down != nil || strings.TrimSpace(m.DownSQL) != ""
}

// applyUp runs the up direction inside tx.
func (m Migration) applyUp(ctx context.Context, tx *sql.Tx) error {
	if m.Up != nil {
		return m.Up(ctx, tx)
	}
	return execScript(ctx, tx, m.UpSQL)
}

// applyDown runs the down direction inside tx.
func (m Migration) applyDown(ctx context.Context, tx *sql.Tx) error {
	if m.Down != nil {
		return m.Down(ctx, tx)
	}
	if strings.TrimSpace(m.DownSQL) == "" {
		return fmt.Errorf("%w: version %d %q is irreversible", ErrMissingMigration, m.Version, m.Name)
	}
	return execScript(ctx, tx, m.DownSQL)
}

// execScript executes each non-empty, semicolon-separated statement in script.
func execScript(ctx context.Context, tx *sql.Tx, script string) error {
	for _, stmt := range splitStatements(script) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec %q: %w", truncate(stmt, 60), err)
		}
	}
	return nil
}

// splitStatements splits a SQL script on semicolons, trimming whitespace and
// dropping empty fragments. It is intentionally simple and does not attempt to
// parse string literals or dollar-quoted bodies.
func splitStatements(script string) []string {
	parts := strings.Split(script, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

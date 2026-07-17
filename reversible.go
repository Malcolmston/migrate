package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// changeCmd is one recorded schema operation with its forward statements and,
// when the operation is reversible, its inverse statements.
type changeCmd struct {
	up           []string
	down         []string // valid only when !irreversible
	irreversible bool
	reason       string
}

// ChangeRecorder captures a sequence of schema operations so a single "change"
// definition can be run forward and, when every operation is reversible,
// automatically reversed. It mirrors ActiveRecord's reversible migrations: you
// describe the change once, and the inverse is derived from it.
//
// Operations that cannot be mechanically inverted (a raw [ChangeRecorder.Execute],
// a [ChangeRecorder.ChangeColumn], or a [ChangeRecorder.DropTable] without a
// rebuild block) mark the change irreversible; attempting to roll such a change
// back fails with [ErrIrreversibleMigration].
type ChangeRecorder struct {
	schema *Schema
	cmds   []changeCmd
}

// Dialect returns the dialect the recorder renders statements for.
func (r *ChangeRecorder) Dialect() Dialect { return r.schema.dialect }

func (r *ChangeRecorder) record(up, down []string) {
	r.cmds = append(r.cmds, changeCmd{up: up, down: down})
}

func (r *ChangeRecorder) recordIrreversible(up []string, reason string) {
	r.cmds = append(r.cmds, changeCmd{up: up, irreversible: true, reason: reason})
}

// CreateTable records a table creation; its inverse is DROP TABLE.
func (r *ChangeRecorder) CreateTable(name string, build func(t *Table), opts ...TableOption) {
	r.record([]string{r.schema.CreateTable(name, build, opts...)}, []string{r.schema.DropTable(name)})
}

// DropTable records a table drop. It is reversible only when build is non-nil
// (describing how to recreate the table); otherwise the change is irreversible.
func (r *ChangeRecorder) DropTable(name string, build func(t *Table), opts ...TableOption) {
	if build == nil {
		r.recordIrreversible([]string{r.schema.DropTable(name)},
			fmt.Sprintf("drop_table %q without a rebuild block", name))
		return
	}
	r.record([]string{r.schema.DropTable(name)}, []string{r.schema.CreateTable(name, build, opts...)})
}

// AddColumn records adding a column; its inverse is DROP COLUMN.
func (r *ChangeRecorder) AddColumn(table, name, sqlType string, opts ...ColumnOption) {
	r.record([]string{r.schema.AddColumn(table, name, sqlType, opts...)},
		[]string{r.schema.DropColumn(table, name)})
}

// RemoveColumn records dropping a column. It is reversible only when sqlType is
// given (so the column can be re-added); otherwise the change is irreversible.
func (r *ChangeRecorder) RemoveColumn(table, name, sqlType string, opts ...ColumnOption) {
	if strings.TrimSpace(sqlType) == "" {
		r.recordIrreversible([]string{r.schema.DropColumn(table, name)},
			fmt.Sprintf("remove_column %q.%q without a type", table, name))
		return
	}
	r.record([]string{r.schema.DropColumn(table, name)},
		[]string{r.schema.AddColumn(table, name, sqlType, opts...)})
}

// RenameColumn records a column rename; its inverse renames back.
func (r *ChangeRecorder) RenameColumn(table, from, to string) {
	r.record([]string{r.schema.RenameColumn(table, from, to)},
		[]string{r.schema.RenameColumn(table, to, from)})
}

// RenameTable records a table rename; its inverse renames back.
func (r *ChangeRecorder) RenameTable(from, to string) {
	r.record([]string{r.schema.RenameTable(from, to)}, []string{r.schema.RenameTable(to, from)})
}

// AddIndex records adding an index; its inverse drops it by name.
func (r *ChangeRecorder) AddIndex(table string, columns []string, opts ...IndexOption) {
	var o indexOptions
	for _, opt := range opts {
		opt(&o)
	}
	name := o.name
	if name == "" {
		name = "index_" + table + "_on_" + strings.Join(sanitizeAll(columns), "_")
	}
	r.record([]string{r.schema.AddIndex(table, columns, opts...)}, []string{r.schema.DropIndex(name)})
}

// AddReference records adding a reference column; its inverse removes it.
func (r *ChangeRecorder) AddReference(table, name string, opts ...ReferenceOption) {
	r.record(splitStatements(r.schema.AddReference(table, name, opts...)),
		[]string{r.schema.RemoveReference(table, name, opts...)})
}

// AddForeignKey records adding a foreign key; its inverse removes it.
func (r *ChangeRecorder) AddForeignKey(fromTable, toTable string, opts ...ForeignKeyOption) {
	r.record([]string{r.schema.AddForeignKey(fromTable, toTable, opts...)},
		[]string{r.schema.RemoveForeignKey(fromTable, toTable, opts...)})
}

// AddTimestamps records adding created_at/updated_at; its inverse removes them.
func (r *ChangeRecorder) AddTimestamps(table string) {
	r.record(splitStatements(r.schema.AddTimestamps(table)),
		splitStatements(r.schema.RemoveTimestamps(table)))
}

// ChangeColumn records a column type change. Because the prior type is not
// captured, the change is irreversible.
func (r *ChangeRecorder) ChangeColumn(table, name, sqlType string, opts ...ColumnOption) {
	r.recordIrreversible([]string{r.schema.ChangeColumn(table, name, sqlType, opts...)},
		fmt.Sprintf("change_column %q.%q", table, name))
}

// Execute records a raw SQL statement. Raw SQL cannot be inverted, so the change
// is irreversible.
func (r *ChangeRecorder) Execute(sql string) {
	r.recordIrreversible([]string{sql}, "raw Execute")
}

// upStatements returns every forward statement in record order.
func (r *ChangeRecorder) upStatements() []string {
	var out []string
	for _, c := range r.cmds {
		out = append(out, c.up...)
	}
	return out
}

// downStatements returns the inverse statements in reverse record order, or an
// error if any recorded operation is irreversible.
func (r *ChangeRecorder) downStatements() ([]string, error) {
	for _, c := range r.cmds {
		if c.irreversible {
			return nil, fmt.Errorf("%w: %s", ErrIrreversibleMigration, c.reason)
		}
	}
	var out []string
	for i := len(r.cmds) - 1; i >= 0; i-- {
		out = append(out, r.cmds[i].down...)
	}
	return out, nil
}

// Reversible reports whether every recorded operation can be inverted.
func (r *ChangeRecorder) Reversible() bool {
	_, err := r.downStatements()
	return err == nil
}

// Change builds a reversible [Migration] from a change definition, using the
// [ANSI] dialect. See [ChangeWith] for the dialect-aware form.
func Change(version uint64, name string, fn func(r *ChangeRecorder)) Migration {
	return ChangeWith(ANSI, version, name, fn)
}

// ChangeWith builds a reversible [Migration] from a change definition rendered
// for dialect d. The migration's Up runs the recorded operations forward; its
// Down runs the automatically derived inverse in reverse order. If any recorded
// operation is irreversible, rolling the migration back fails with
// [ErrIrreversibleMigration].
func ChangeWith(d Dialect, version uint64, name string, fn func(r *ChangeRecorder)) Migration {
	r := &ChangeRecorder{schema: NewSchema(d)}
	if fn != nil {
		fn(r)
	}
	up := r.upStatements()
	down, downErr := r.downStatements()

	m := Migration{Version: version, Name: name}
	m.Up = func(ctx context.Context, tx *sql.Tx) error {
		return execStatements(ctx, tx, up)
	}
	// Always install a Down so the migrator treats the change as reversible and
	// surfaces a clear error for irreversible ones instead of a generic
	// "missing migration".
	m.Down = func(ctx context.Context, tx *sql.Tx) error {
		if downErr != nil {
			return downErr
		}
		return execStatements(ctx, tx, down)
	}
	return m
}

// execStatements runs each statement inside tx in order.
func execStatements(ctx context.Context, tx *sql.Tx, stmts []string) error {
	for _, s := range stmts {
		if strings.TrimSpace(s) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("exec %q: %w", truncate(s, 60), err)
		}
	}
	return nil
}

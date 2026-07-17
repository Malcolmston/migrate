package migrate

import "strings"

// AlterTable accumulates alterations inside a [Schema.ChangeTable] block and
// renders them as a sequence of ALTER TABLE / CREATE INDEX statements. It
// mirrors ActiveRecord's change_table bulk-alter builder: each call appends one
// operation, and the whole block is emitted as ";\n"-joined statements in call
// order.
type AlterTable struct {
	schema *Schema
	table  string
	stmts  []string
}

// column helpers -----------------------------------------------------------

// Column adds a column with an explicit SQL type spelling.
func (a *AlterTable) Column(name, sqlType string, opts ...ColumnOption) {
	a.stmts = append(a.stmts, a.schema.AddColumn(a.table, name, sqlType, opts...))
}

// String adds a VARCHAR column.
func (a *AlterTable) String(name string, opts ...ColumnOption) { a.Column(name, "VARCHAR", opts...) }

// Text adds a TEXT column.
func (a *AlterTable) Text(name string, opts ...ColumnOption) { a.Column(name, "TEXT", opts...) }

// Integer adds an INTEGER column.
func (a *AlterTable) Integer(name string, opts ...ColumnOption) { a.Column(name, "INTEGER", opts...) }

// BigInteger adds a BIGINT column.
func (a *AlterTable) BigInteger(name string, opts ...ColumnOption) { a.Column(name, "BIGINT", opts...) }

// Boolean adds a BOOLEAN column.
func (a *AlterTable) Boolean(name string, opts ...ColumnOption) { a.Column(name, "BOOLEAN", opts...) }

// Timestamp adds a TIMESTAMP column.
func (a *AlterTable) Timestamp(name string, opts ...ColumnOption) {
	a.Column(name, "TIMESTAMP", opts...)
}

// Timestamps adds created_at and updated_at NOT NULL timestamp columns.
func (a *AlterTable) Timestamps() {
	a.stmts = append(a.stmts,
		a.schema.AddColumn(a.table, "created_at", "TIMESTAMP", NotNull()),
		a.schema.AddColumn(a.table, "updated_at", "TIMESTAMP", NotNull()))
}

// References adds a reference column (see [Schema.AddReference]).
func (a *AlterTable) References(name string, opts ...ReferenceOption) {
	a.stmts = append(a.stmts, splitStatements(a.schema.AddReference(a.table, name, opts...))...)
}

// mutation helpers ----------------------------------------------------------

// Remove drops one or more columns.
func (a *AlterTable) Remove(names ...string) {
	for _, n := range names {
		a.stmts = append(a.stmts, a.schema.DropColumn(a.table, n))
	}
}

// Rename renames a column.
func (a *AlterTable) Rename(from, to string) {
	a.stmts = append(a.stmts, a.schema.RenameColumn(a.table, from, to))
}

// Change alters the type of an existing column.
func (a *AlterTable) Change(name, sqlType string, opts ...ColumnOption) {
	a.stmts = append(a.stmts, a.schema.ChangeColumn(a.table, name, sqlType, opts...))
}

// Index adds an index over columns (see [Schema.AddIndex]).
func (a *AlterTable) Index(columns []string, opts ...IndexOption) {
	a.stmts = append(a.stmts, a.schema.AddIndex(a.table, columns, opts...))
}

// RemoveIndex drops a named index.
func (a *AlterTable) RemoveIndex(name string) {
	a.stmts = append(a.stmts, a.schema.DropIndex(name))
}

// ChangeTable builds a bulk table alteration. The build callback receives an
// [AlterTable]; every operation it records is emitted in order as ";\n"-joined
// statements.
func (s *Schema) ChangeTable(name string, build func(t *AlterTable)) string {
	a := &AlterTable{schema: s, table: name}
	if build != nil {
		build(a)
	}
	return strings.Join(a.stmts, ";\n")
}

// package-level convenience wrappers (ANSI dialect) -------------------------

// ChangeTable builds a bulk table alteration using the [ANSI] dialect.
func ChangeTable(name string, build func(t *AlterTable)) string {
	return ansiSchema.ChangeTable(name, build)
}

// ChangeColumn builds a column type change using the [ANSI] dialect.
func ChangeColumn(table, name, sqlType string, opts ...ColumnOption) string {
	return ansiSchema.ChangeColumn(table, name, sqlType, opts...)
}

// AddReference builds a reference-column addition using the [ANSI] dialect.
func AddReference(table, name string, opts ...ReferenceOption) string {
	return ansiSchema.AddReference(table, name, opts...)
}

// RemoveReference builds a reference-column removal using the [ANSI] dialect.
func RemoveReference(table, name string, opts ...ReferenceOption) string {
	return ansiSchema.RemoveReference(table, name, opts...)
}

// AddForeignKey builds a foreign-key constraint using the [ANSI] dialect.
func AddForeignKey(fromTable, toTable string, opts ...ForeignKeyOption) string {
	return ansiSchema.AddForeignKey(fromTable, toTable, opts...)
}

// RemoveForeignKey builds a foreign-key removal using the [ANSI] dialect.
func RemoveForeignKey(fromTable, toTable string, opts ...ForeignKeyOption) string {
	return ansiSchema.RemoveForeignKey(fromTable, toTable, opts...)
}

// AddTimestamps builds created_at/updated_at additions using the [ANSI] dialect.
func AddTimestamps(table string) string { return ansiSchema.AddTimestamps(table) }

// RemoveTimestamps builds created_at/updated_at removals using the [ANSI]
// dialect.
func RemoveTimestamps(table string) string { return ansiSchema.RemoveTimestamps(table) }

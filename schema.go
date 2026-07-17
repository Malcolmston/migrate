package migrate

import (
	"fmt"
	"strings"
)

// This file implements a small ActiveRecord-flavoured schema DSL. Every helper
// returns a SQL string (or strings joined by ";\n") targeting the conservative
// ANSI dialect documented in the package overview. Nothing here touches a
// database; the output is meant to be placed in a migration's UpSQL/DownSQL or
// executed by a Go migration.

// colType is a portable ANSI column type spelling.
type colType string

// Portable column type spellings used by the DSL.
const (
	typeString  colType = "VARCHAR"          // parameterised by Limit, defaults to VARCHAR(255)
	typeText    colType = "TEXT"             //
	typeInteger colType = "INTEGER"          //
	typeBigInt  colType = "BIGINT"           //
	typeBoolean colType = "BOOLEAN"          //
	typeFloat   colType = "DOUBLE PRECISION" //
	typeTime    colType = "TIMESTAMP"        //
	typeDate    colType = "DATE"             //
)

// colOptions holds the resolved per-column modifiers.
type colOptions struct {
	notNull    bool
	primaryKey bool
	unique     bool
	hasDefault bool
	defaultSQL string
	limit      int
}

// ColumnOption modifies a column definition.
type ColumnOption func(*colOptions)

// NotNull marks the column NOT NULL. Columns are nullable by default.
func NotNull() ColumnOption { return func(o *colOptions) { o.notNull = true } }

// PrimaryKey marks the column as the table's PRIMARY KEY.
func PrimaryKey() ColumnOption { return func(o *colOptions) { o.primaryKey = true } }

// Unique adds a UNIQUE constraint to the column.
func Unique() ColumnOption { return func(o *colOptions) { o.unique = true } }

// Limit sets the length for VARCHAR columns (ignored by other types).
func Limit(n int) ColumnOption { return func(o *colOptions) { o.limit = n } }

// Default sets a column default. String values are emitted as quoted SQL
// literals; all other values use their default Go formatting (numbers, bools).
// Use [DefaultRaw] for expressions such as CURRENT_TIMESTAMP.
func Default(v any) ColumnOption {
	return func(o *colOptions) {
		o.hasDefault = true
		switch t := v.(type) {
		case string:
			o.defaultSQL = quote(t)
		case bool:
			if t {
				o.defaultSQL = "TRUE"
			} else {
				o.defaultSQL = "FALSE"
			}
		default:
			o.defaultSQL = fmt.Sprintf("%v", v)
		}
	}
}

// DefaultRaw sets a column default to a raw SQL expression, emitted verbatim.
func DefaultRaw(expr string) ColumnOption {
	return func(o *colOptions) {
		o.hasDefault = true
		o.defaultSQL = expr
	}
}

// column is a single resolved column definition.
type column struct {
	name string
	typ  colType
	opts colOptions
}

// sql renders the column as it appears inside a CREATE TABLE statement.
func (c column) sql() string {
	var b strings.Builder
	b.WriteString(c.name)
	b.WriteByte(' ')
	b.WriteString(c.typeSQL())
	if c.opts.primaryKey {
		b.WriteString(" PRIMARY KEY")
	}
	if c.opts.notNull && !c.opts.primaryKey {
		b.WriteString(" NOT NULL")
	}
	if c.opts.unique && !c.opts.primaryKey {
		b.WriteString(" UNIQUE")
	}
	if c.opts.hasDefault {
		b.WriteString(" DEFAULT ")
		b.WriteString(c.opts.defaultSQL)
	}
	return b.String()
}

func (c column) typeSQL() string {
	if c.typ == typeString {
		n := c.opts.limit
		if n <= 0 {
			n = 255
		}
		return fmt.Sprintf("VARCHAR(%d)", n)
	}
	return string(c.typ)
}

// Table accumulates column definitions inside a [CreateTable] block.
type Table struct {
	name        string
	columns     []column
	foreignKeys []string
}

func (t *Table) add(name string, typ colType, opts ...ColumnOption) {
	var o colOptions
	for _, opt := range opts {
		opt(&o)
	}
	t.columns = append(t.columns, column{name: name, typ: typ, opts: o})
}

// Column adds a column with an explicit ANSI type spelling.
func (t *Table) Column(name, sqlType string, opts ...ColumnOption) {
	t.add(name, colType(sqlType), opts...)
}

// String adds a VARCHAR column (VARCHAR(255) unless [Limit] is given).
func (t *Table) String(name string, opts ...ColumnOption) { t.add(name, typeString, opts...) }

// Text adds a TEXT column.
func (t *Table) Text(name string, opts ...ColumnOption) { t.add(name, typeText, opts...) }

// Integer adds an INTEGER column.
func (t *Table) Integer(name string, opts ...ColumnOption) { t.add(name, typeInteger, opts...) }

// BigInteger adds a BIGINT column.
func (t *Table) BigInteger(name string, opts ...ColumnOption) { t.add(name, typeBigInt, opts...) }

// Boolean adds a BOOLEAN column.
func (t *Table) Boolean(name string, opts ...ColumnOption) { t.add(name, typeBoolean, opts...) }

// Float adds a DOUBLE PRECISION column.
func (t *Table) Float(name string, opts ...ColumnOption) { t.add(name, typeFloat, opts...) }

// Timestamp adds a single TIMESTAMP column.
func (t *Table) Timestamp(name string, opts ...ColumnOption) { t.add(name, typeTime, opts...) }

// Date adds a DATE column.
func (t *Table) Date(name string, opts ...ColumnOption) { t.add(name, typeDate, opts...) }

// Timestamps adds the conventional created_at / updated_at NOT NULL TIMESTAMP
// columns.
func (t *Table) Timestamps() {
	t.add("created_at", typeTime, NotNull())
	t.add("updated_at", typeTime, NotNull())
}

// References adds a "<name>_id" BIGINT foreign-key column. By default it only
// adds the column; pass [WithForeignKey] to also emit an inline REFERENCES
// constraint to the pluralised table's id column.
func (t *Table) References(name string, opts ...ReferenceOption) {
	ro := referenceOptions{column: name + "_id", refTable: pluralize(name), refColumn: "id"}
	for _, opt := range opts {
		opt(&ro)
	}
	colOpts := []ColumnOption{}
	if ro.notNull {
		colOpts = append(colOpts, NotNull())
	}
	t.add(ro.column, typeBigInt, colOpts...)
	if ro.foreignKey {
		t.foreignKeys = append(t.foreignKeys,
			fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s)", ro.column, ro.refTable, ro.refColumn))
	}
}

// referenceOptions holds resolved [Table.References] modifiers.
type referenceOptions struct {
	column     string
	refTable   string
	refColumn  string
	foreignKey bool
	notNull    bool
}

// ReferenceOption modifies a [Table.References] column.
type ReferenceOption func(*referenceOptions)

// WithForeignKey emits an inline FOREIGN KEY constraint for the reference.
func WithForeignKey() ReferenceOption { return func(o *referenceOptions) { o.foreignKey = true } }

// ReferenceNotNull marks the reference column NOT NULL.
func ReferenceNotNull() ReferenceOption { return func(o *referenceOptions) { o.notNull = true } }

// ReferenceTable overrides the referenced table name (default: pluralised).
func ReferenceTable(name string) ReferenceOption {
	return func(o *referenceOptions) { o.refTable = name }
}

// tableOptions holds resolved [CreateTable] modifiers.
type tableOptions struct {
	withoutID   bool
	ifNotExists bool
}

// TableOption modifies a [CreateTable] statement.
type TableOption func(*tableOptions)

// WithoutID suppresses the automatic identity primary key column.
func WithoutID() TableOption { return func(o *tableOptions) { o.withoutID = true } }

// IfNotExists adds IF NOT EXISTS to the CREATE TABLE statement.
func IfNotExists() TableOption { return func(o *tableOptions) { o.ifNotExists = true } }

// CreateTable builds a CREATE TABLE statement. Unless [WithoutID] is supplied it
// prepends an auto-incrementing "id BIGINT GENERATED BY DEFAULT AS IDENTITY
// PRIMARY KEY" column, matching the ActiveRecord convention.
func CreateTable(name string, build func(t *Table), opts ...TableOption) string {
	var o tableOptions
	for _, opt := range opts {
		opt(&o)
	}

	t := &Table{name: name}
	if build != nil {
		build(t)
	}

	lines := make([]string, 0, len(t.columns)+1)
	if !o.withoutID {
		lines = append(lines, "id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY")
	}
	for _, c := range t.columns {
		lines = append(lines, c.sql())
	}
	lines = append(lines, t.foreignKeys...)

	head := "CREATE TABLE "
	if o.ifNotExists {
		head = "CREATE TABLE IF NOT EXISTS "
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString(name)
	b.WriteString(" (\n")
	for i, l := range lines {
		b.WriteString("\t")
		b.WriteString(l)
		if i < len(lines)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteByte(')')
	return b.String()
}

// DropTable builds a DROP TABLE statement.
func DropTable(name string) string {
	return "DROP TABLE " + name
}

// DropTableIfExists builds a DROP TABLE IF EXISTS statement.
func DropTableIfExists(name string) string {
	return "DROP TABLE IF EXISTS " + name
}

// RenameTable builds an ALTER TABLE ... RENAME TO statement.
func RenameTable(from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME TO %s", from, to)
}

// AddColumn builds an ALTER TABLE ... ADD COLUMN statement using an explicit
// ANSI type spelling. For DSL type helpers use the [Table] methods within
// [CreateTable] instead.
func AddColumn(table, name, sqlType string, opts ...ColumnOption) string {
	var o colOptions
	for _, opt := range opts {
		opt(&o)
	}
	c := column{name: name, typ: colType(sqlType), opts: o}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, c.sql())
}

// DropColumn builds an ALTER TABLE ... DROP COLUMN statement.
func DropColumn(table, name string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, name)
}

// RenameColumn builds an ALTER TABLE ... RENAME COLUMN ... TO ... statement.
func RenameColumn(table, from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", table, from, to)
}

// indexOptions holds resolved [AddIndex] modifiers.
type indexOptions struct {
	unique bool
	name   string
}

// IndexOption modifies an [AddIndex] statement.
type IndexOption func(*indexOptions)

// UniqueIndex makes the index UNIQUE.
func UniqueIndex() IndexOption { return func(o *indexOptions) { o.unique = true } }

// IndexName overrides the generated index name.
func IndexName(name string) IndexOption { return func(o *indexOptions) { o.name = name } }

// AddIndex builds a CREATE INDEX statement over the given columns. When no name
// is supplied the ActiveRecord-style "index_<table>_on_<cols>" convention is
// used.
func AddIndex(table string, columns []string, opts ...IndexOption) string {
	var o indexOptions
	for _, opt := range opts {
		opt(&o)
	}
	name := o.name
	if name == "" {
		name = "index_" + table + "_on_" + strings.Join(columns, "_")
	}
	kw := "CREATE INDEX"
	if o.unique {
		kw = "CREATE UNIQUE INDEX"
	}
	return fmt.Sprintf("%s %s ON %s (%s)", kw, name, table, strings.Join(columns, ", "))
}

// DropIndex builds a DROP INDEX statement.
func DropIndex(name string) string {
	return "DROP INDEX " + name
}

// quote renders a SQL string literal, escaping embedded single quotes.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// pluralize is a deliberately naive English pluraliser: it lowercases nothing
// and simply appends "s" unless the word already ends in one. Override the
// referenced table name with [ReferenceTable] when this is wrong.
func pluralize(s string) string {
	if strings.HasSuffix(s, "s") {
		return s
	}
	return s + "s"
}

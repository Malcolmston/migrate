package migrate

import (
	"fmt"
	"strings"
)

// This file implements an ActiveRecord-flavoured schema DSL. Every helper
// returns a SQL string (or statements joined by ";\n"). The concrete SQL is
// produced by a [Dialect]; the package-level helpers use the [ANSI] dialect so
// their output matches the conservative portable spelling documented in the
// package overview. Bind a different dialect with [NewSchema] to target
// PostgreSQL, MySQL, or SQLite. Nothing here touches a database; the output is
// meant to be placed in a migration's UpSQL/DownSQL or executed by a Go
// migration.

// typeKind enumerates the abstract column types understood by the DSL. Each
// kind is mapped to a concrete SQL spelling by a [Dialect].
type typeKind int

const (
	kindRaw       typeKind = iota // verbatim SQL type text (from Column/AddColumn)
	kindString                    // VARCHAR(limit)
	kindText                      // TEXT / long character data
	kindInteger                   // 32-bit integer
	kindBigInt                    // 64-bit integer
	kindFloat                     // double-precision floating point
	kindBoolean                   // boolean
	kindDecimal                   // fixed-point DECIMAL(precision, scale)
	kindTimestamp                 // date + time, optional precision / time zone
	kindDate                      // calendar date
	kindTime                      // time of day
	kindBinary                    // binary large object
	kindJSON                      // textual JSON document
	kindJSONB                     // binary JSON document (PostgreSQL jsonb)
	kindUUID                      // universally unique identifier
	kindEnum                      // enumerated string type
)

// typeSpec is the resolved, dialect-independent description of a column type.
// A [Dialect] renders it to concrete SQL via its columnType method.
type typeSpec struct {
	kind      typeKind
	raw       string   // literal type text when kind == kindRaw
	limit     int      // VARCHAR length
	precision int      // DECIMAL/timestamp precision
	scale     int      // DECIMAL scale
	withTZ    bool     // timestamp WITH TIME ZONE
	array     bool     // array of the base type
	enumName  string   // named enum type (PostgreSQL) or logical label
	enumVals  []string // permitted enum values
}

// colOptions holds the resolved per-column modifiers.
type colOptions struct {
	notNull    bool
	primaryKey bool
	unique     bool
	hasDefault bool
	defaultSQL string
	limit      int
	precision  int
	scale      int
	withTZ     bool
	array      bool
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

// Precision sets the total number of digits for DECIMAL columns or the
// fractional-second precision for TIMESTAMP/TIME columns.
func Precision(n int) ColumnOption { return func(o *colOptions) { o.precision = n } }

// Scale sets the number of digits to the right of the decimal point for DECIMAL
// columns.
func Scale(n int) ColumnOption { return func(o *colOptions) { o.scale = n } }

// WithTimezone renders a TIMESTAMP/TIME column as WITH TIME ZONE where the
// dialect supports it.
func WithTimezone() ColumnOption { return func(o *colOptions) { o.withTZ = true } }

// Array marks the column as an array of its base type. Native arrays are only
// emitted for the PostgreSQL and ANSI dialects; MySQL and SQLite fall back to a
// JSON column.
func Array() ColumnOption { return func(o *colOptions) { o.array = true } }

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
	spec typeSpec
	opts colOptions
}

// render renders the column as it appears inside a CREATE TABLE statement for
// the given dialect.
func (c column) render(d Dialect) string {
	var b strings.Builder
	b.WriteString(d.Quote(c.name))
	b.WriteByte(' ')
	b.WriteString(d.columnType(c.spec))
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

// Table accumulates column definitions inside a [Schema.CreateTable] block.
type Table struct {
	name        string
	dialect     Dialect
	columns     []column
	foreignKeys []string
}

// addSpec resolves the column options, folds any type parameters carried by the
// options into the type spec, and appends the column.
func (t *Table) addSpec(name string, spec typeSpec, opts ...ColumnOption) {
	var o colOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.limit > 0 && spec.limit == 0 {
		spec.limit = o.limit
	}
	if o.precision > 0 && spec.precision == 0 {
		spec.precision = o.precision
	}
	if o.scale > 0 && spec.scale == 0 {
		spec.scale = o.scale
	}
	if o.withTZ {
		spec.withTZ = true
	}
	if o.array {
		spec.array = true
	}
	t.columns = append(t.columns, column{name: name, spec: spec, opts: o})
}

// rawSpec maps a verbatim SQL type name to a spec. The historically special
// "VARCHAR"/"STRING" spellings become a length-aware string type so that
// [Limit] keeps working; everything else is emitted verbatim.
func rawSpec(sqlType string) typeSpec {
	switch strings.ToUpper(strings.TrimSpace(sqlType)) {
	case "VARCHAR", "STRING":
		return typeSpec{kind: kindString}
	default:
		return typeSpec{kind: kindRaw, raw: sqlType}
	}
}

// Column adds a column with an explicit SQL type spelling. For DSL type helpers
// use the typed [Table] methods instead.
func (t *Table) Column(name, sqlType string, opts ...ColumnOption) {
	t.addSpec(name, rawSpec(sqlType), opts...)
}

// String adds a VARCHAR column (VARCHAR(255) unless [Limit] is given).
func (t *Table) String(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindString}, opts...)
}

// Text adds a TEXT column.
func (t *Table) Text(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindText}, opts...)
}

// Integer adds a 32-bit integer column.
func (t *Table) Integer(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindInteger}, opts...)
}

// BigInteger adds a 64-bit integer column.
func (t *Table) BigInteger(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindBigInt}, opts...)
}

// Boolean adds a boolean column.
func (t *Table) Boolean(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindBoolean}, opts...)
}

// Float adds a double-precision floating point column.
func (t *Table) Float(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindFloat}, opts...)
}

// Decimal adds a fixed-point DECIMAL(precision, scale) column. A precision of
// zero omits the size specifier entirely.
func (t *Table) Decimal(name string, precision, scale int, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindDecimal, precision: precision, scale: scale}, opts...)
}

// Timestamp adds a single date-and-time column. Combine with [Precision] and
// [WithTimezone] for fractional-second precision and time-zone awareness.
func (t *Table) Timestamp(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindTimestamp}, opts...)
}

// Date adds a calendar-date column.
func (t *Table) Date(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindDate}, opts...)
}

// Time adds a time-of-day column.
func (t *Table) Time(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindTime}, opts...)
}

// Binary adds a binary large object column.
func (t *Table) Binary(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindBinary}, opts...)
}

// JSON adds a textual JSON column.
func (t *Table) JSON(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindJSON}, opts...)
}

// JSONB adds a binary JSON column (PostgreSQL jsonb; other dialects fall back to
// their JSON type).
func (t *Table) JSONB(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindJSONB}, opts...)
}

// UUID adds a universally-unique-identifier column.
func (t *Table) UUID(name string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindUUID}, opts...)
}

// Enum adds an enumerated string column restricted to values. On PostgreSQL the
// column references a named type (see [Table.EnumType]); MySQL emits an inline
// ENUM; other dialects fall back to VARCHAR.
func (t *Table) Enum(name string, values []string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindEnum, enumVals: values, enumName: name}, opts...)
}

// EnumType adds an enumerated string column that references a pre-declared
// PostgreSQL type named typeName.
func (t *Table) EnumType(name, typeName string, values []string, opts ...ColumnOption) {
	t.addSpec(name, typeSpec{kind: kindEnum, enumVals: values, enumName: typeName}, opts...)
}

// Timestamps adds the conventional created_at / updated_at NOT NULL timestamp
// columns.
func (t *Table) Timestamps() {
	t.addSpec("created_at", typeSpec{kind: kindTimestamp}, NotNull())
	t.addSpec("updated_at", typeSpec{kind: kindTimestamp}, NotNull())
}

// References adds a "<name>_id" foreign-key column. By default it only adds the
// column; pass [WithForeignKey] to also emit an inline REFERENCES constraint to
// the pluralised table's id column.
func (t *Table) References(name string, opts ...ReferenceOption) {
	ro := referenceOptions{column: name + "_id", refTable: pluralize(name), refColumn: "id"}
	for _, opt := range opts {
		opt(&ro)
	}
	var colOpts []ColumnOption
	if ro.notNull {
		colOpts = append(colOpts, NotNull())
	}
	t.addSpec(ro.column, typeSpec{kind: kindBigInt}, colOpts...)
	if ro.foreignKey {
		t.foreignKeys = append(t.foreignKeys,
			fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s)",
				t.dialect.Quote(ro.column), t.dialect.Quote(ro.refTable), t.dialect.Quote(ro.refColumn)))
	}
}

// referenceOptions holds resolved [Table.References] modifiers.
type referenceOptions struct {
	column     string
	refTable   string
	refColumn  string
	foreignKey bool
	notNull    bool
	index      bool
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

// ReferenceIndex adds an index on the reference column. It applies to
// [Schema.AddReference]; inline table references ignore it.
func ReferenceIndex() ReferenceOption { return func(o *referenceOptions) { o.index = true } }

// tableOptions holds resolved CreateTable modifiers.
type tableOptions struct {
	withoutID   bool
	ifNotExists bool
}

// TableOption modifies a CreateTable statement.
type TableOption func(*tableOptions)

// WithoutID suppresses the automatic identity primary key column.
func WithoutID() TableOption { return func(o *tableOptions) { o.withoutID = true } }

// IfNotExists adds IF NOT EXISTS to the CREATE TABLE statement.
func IfNotExists() TableOption { return func(o *tableOptions) { o.ifNotExists = true } }

// CreateTable builds a CREATE TABLE statement using the [ANSI] dialect. See
// [Schema.CreateTable] for the dialect-aware form.
func CreateTable(name string, build func(t *Table), opts ...TableOption) string {
	return ansiSchema.CreateTable(name, build, opts...)
}

// DropTable builds a DROP TABLE statement using the [ANSI] dialect.
func DropTable(name string) string { return ansiSchema.DropTable(name) }

// DropTableIfExists builds a DROP TABLE IF EXISTS statement using the [ANSI]
// dialect.
func DropTableIfExists(name string) string { return ansiSchema.DropTableIfExists(name) }

// RenameTable builds an ALTER TABLE ... RENAME TO statement using the [ANSI]
// dialect.
func RenameTable(from, to string) string { return ansiSchema.RenameTable(from, to) }

// AddColumn builds an ALTER TABLE ... ADD COLUMN statement using the [ANSI]
// dialect and an explicit SQL type spelling.
func AddColumn(table, name, sqlType string, opts ...ColumnOption) string {
	return ansiSchema.AddColumn(table, name, sqlType, opts...)
}

// DropColumn builds an ALTER TABLE ... DROP COLUMN statement using the [ANSI]
// dialect.
func DropColumn(table, name string) string { return ansiSchema.DropColumn(table, name) }

// RenameColumn builds an ALTER TABLE ... RENAME COLUMN statement using the
// [ANSI] dialect.
func RenameColumn(table, from, to string) string { return ansiSchema.RenameColumn(table, from, to) }

// AddIndex builds a CREATE INDEX statement using the [ANSI] dialect.
func AddIndex(table string, columns []string, opts ...IndexOption) string {
	return ansiSchema.AddIndex(table, columns, opts...)
}

// DropIndex builds a DROP INDEX statement using the [ANSI] dialect.
func DropIndex(name string) string { return ansiSchema.DropIndex(name) }

// quote renders a SQL string literal, escaping embedded single quotes.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// pluralize is a deliberately naive English pluraliser: it simply appends "s"
// unless the word already ends in one. Override the referenced table name with
// [ReferenceTable] when this is wrong.
func pluralize(s string) string {
	if strings.HasSuffix(s, "s") {
		return s
	}
	return s + "s"
}

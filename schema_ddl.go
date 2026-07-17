package migrate

import (
	"fmt"
	"strings"
)

// Schema is a dialect-aware builder for schema DSL statements. Every method
// returns a SQL string (or ";\n"-joined statements) rendered for the bound
// [Dialect]. The package-level helpers ([CreateTable], [AddColumn], ...) are
// thin wrappers around a Schema bound to [ANSI].
type Schema struct {
	dialect Dialect
}

// ansiSchema is the default builder backing the package-level DSL helpers.
var ansiSchema = &Schema{dialect: ANSI}

// NewSchema returns a [Schema] that renders statements for d. A nil dialect
// defaults to [ANSI].
func NewSchema(d Dialect) *Schema {
	if d == nil {
		d = ANSI
	}
	return &Schema{dialect: d}
}

// Dialect returns the dialect the schema renders for.
func (s *Schema) Dialect() Dialect { return s.dialect }

// CreateTable builds a CREATE TABLE statement. Unless [WithoutID] is supplied it
// prepends an auto-incrementing primary key column appropriate to the dialect,
// matching the ActiveRecord convention.
func (s *Schema) CreateTable(name string, build func(t *Table), opts ...TableOption) string {
	var o tableOptions
	for _, opt := range opts {
		opt(&o)
	}

	t := &Table{name: name, dialect: s.dialect}
	if build != nil {
		build(t)
	}

	lines := make([]string, 0, len(t.columns)+1)
	if !o.withoutID {
		lines = append(lines, s.dialect.autoIncrementPK("id"))
	}
	for _, c := range t.columns {
		lines = append(lines, c.render(s.dialect))
	}
	lines = append(lines, t.foreignKeys...)

	head := "CREATE TABLE "
	if o.ifNotExists {
		head = "CREATE TABLE IF NOT EXISTS "
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString(s.dialect.Quote(name))
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
func (s *Schema) DropTable(name string) string {
	return "DROP TABLE " + s.dialect.Quote(name)
}

// DropTableIfExists builds a DROP TABLE IF EXISTS statement.
func (s *Schema) DropTableIfExists(name string) string {
	return "DROP TABLE IF EXISTS " + s.dialect.Quote(name)
}

// RenameTable builds an ALTER TABLE ... RENAME TO statement.
func (s *Schema) RenameTable(from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME TO %s", s.dialect.Quote(from), s.dialect.Quote(to))
}

// AddColumn builds an ALTER TABLE ... ADD COLUMN statement using an explicit SQL
// type spelling.
func (s *Schema) AddColumn(table, name, sqlType string, opts ...ColumnOption) string {
	t := &Table{name: table, dialect: s.dialect}
	t.addSpec(name, rawSpec(sqlType), opts...)
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s",
		s.dialect.Quote(table), t.columns[0].render(s.dialect))
}

// DropColumn builds an ALTER TABLE ... DROP COLUMN statement.
func (s *Schema) DropColumn(table, name string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", s.dialect.Quote(table), s.dialect.Quote(name))
}

// RenameColumn builds an ALTER TABLE ... RENAME COLUMN ... TO ... statement.
func (s *Schema) RenameColumn(table, from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
		s.dialect.Quote(table), s.dialect.Quote(from), s.dialect.Quote(to))
}

// ChangeColumn builds a statement that changes the type of an existing column.
// The exact syntax depends on the dialect: MySQL emits MODIFY COLUMN, while
// PostgreSQL, SQLite, and ANSI emit ALTER COLUMN ... TYPE. The type is given as
// an explicit SQL spelling.
func (s *Schema) ChangeColumn(table, name, sqlType string, opts ...ColumnOption) string {
	col := &Table{name: table, dialect: s.dialect}
	col.addSpec(name, rawSpec(sqlType), opts...)
	c := col.columns[0]
	typeSQL := s.dialect.columnType(c.spec)
	qt := s.dialect.Quote(table)
	qc := s.dialect.Quote(name)
	if s.dialect.Name() == "mysql" {
		// MySQL restates the full column definition.
		return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s", qt, c.render(s.dialect))
	}
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s", qt, qc, typeSQL)
}

// AddTimestamps builds a statement adding created_at and updated_at NOT NULL
// timestamp columns to an existing table.
func (s *Schema) AddTimestamps(table string) string {
	return strings.Join([]string{
		s.AddColumn(table, "created_at", "TIMESTAMP", NotNull()),
		s.AddColumn(table, "updated_at", "TIMESTAMP", NotNull()),
	}, ";\n")
}

// RemoveTimestamps builds a statement dropping the created_at and updated_at
// columns from a table.
func (s *Schema) RemoveTimestamps(table string) string {
	return strings.Join([]string{
		s.DropColumn(table, "updated_at"),
		s.DropColumn(table, "created_at"),
	}, ";\n")
}

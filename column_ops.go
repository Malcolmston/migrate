package migrate

import "fmt"

// This file adds column-level alteration helpers toward ActiveRecord parity:
// change_column_default, change_column_null, and rename_index. Every helper
// returns a SQL string for the bound [Dialect] and touches no database.

// ddlLiteral renders a Go value as a SQL literal using the same rules as the
// [Default] column option: strings are quoted, booleans become TRUE/FALSE, and
// everything else uses default Go formatting.
func ddlLiteral(v any) string {
	switch t := v.(type) {
	case string:
		return quote(t)
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ChangeColumnDefault builds an ALTER TABLE ... ALTER COLUMN ... SET DEFAULT
// statement. The value is rendered with the same rules as the [Default] column
// option (strings quoted, booleans as TRUE/FALSE). Use
// [Schema.ChangeColumnDefaultRaw] for an expression such as CURRENT_TIMESTAMP.
func (s *Schema) ChangeColumnDefault(table, column string, value any) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s",
		s.dialect.Quote(table), s.dialect.Quote(column), ddlLiteral(value))
}

// ChangeColumnDefaultRaw builds an ALTER TABLE ... ALTER COLUMN ... SET DEFAULT
// statement whose default is the given SQL expression, emitted verbatim.
func (s *Schema) ChangeColumnDefaultRaw(table, column, expr string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s",
		s.dialect.Quote(table), s.dialect.Quote(column), expr)
}

// DropColumnDefault builds an ALTER TABLE ... ALTER COLUMN ... DROP DEFAULT
// statement, removing any default previously set on the column.
func (s *Schema) DropColumnDefault(table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT",
		s.dialect.Quote(table), s.dialect.Quote(column))
}

// ChangeColumnNull builds a statement toggling a column's NOT NULL constraint,
// mirroring ActiveRecord's change_column_null. Passing null=false adds NOT NULL;
// null=true drops it. The standard "ALTER COLUMN ... SET/DROP NOT NULL" spelling
// is emitted (supported by PostgreSQL, SQLite, and ANSI); MySQL requires a full
// column redefinition, so use [Schema.ChangeColumn] there.
func (s *Schema) ChangeColumnNull(table, column string, null bool) string {
	action := "SET NOT NULL"
	if null {
		action = "DROP NOT NULL"
	}
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s %s",
		s.dialect.Quote(table), s.dialect.Quote(column), action)
}

// RenameIndex builds a statement renaming an index, mirroring ActiveRecord's
// rename_index. The syntax is dialect-specific: MySQL uses
// "ALTER TABLE <table> RENAME INDEX <from> TO <to>", while PostgreSQL, SQLite,
// and ANSI use "ALTER INDEX <from> RENAME TO <to>" (the table argument is
// unused there). SQLite has no native rename; drop and recreate instead.
func (s *Schema) RenameIndex(table, from, to string) string {
	if s.dialect.Name() == "mysql" {
		return fmt.Sprintf("ALTER TABLE %s RENAME INDEX %s TO %s",
			s.dialect.Quote(table), s.dialect.Quote(from), s.dialect.Quote(to))
	}
	return fmt.Sprintf("ALTER INDEX %s RENAME TO %s",
		s.dialect.Quote(from), s.dialect.Quote(to))
}

// package-level convenience wrappers (ANSI dialect) -------------------------

// ChangeColumnDefault builds a SET DEFAULT statement using the [ANSI] dialect.
func ChangeColumnDefault(table, column string, value any) string {
	return ansiSchema.ChangeColumnDefault(table, column, value)
}

// ChangeColumnDefaultRaw builds a raw-expression SET DEFAULT statement using the
// [ANSI] dialect.
func ChangeColumnDefaultRaw(table, column, expr string) string {
	return ansiSchema.ChangeColumnDefaultRaw(table, column, expr)
}

// DropColumnDefault builds a DROP DEFAULT statement using the [ANSI] dialect.
func DropColumnDefault(table, column string) string {
	return ansiSchema.DropColumnDefault(table, column)
}

// ChangeColumnNull builds a SET/DROP NOT NULL statement using the [ANSI] dialect.
func ChangeColumnNull(table, column string, null bool) string {
	return ansiSchema.ChangeColumnNull(table, column, null)
}

// RenameIndex builds an index-rename statement using the [ANSI] dialect.
func RenameIndex(table, from, to string) string {
	return ansiSchema.RenameIndex(table, from, to)
}

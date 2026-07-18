package migrate

import (
	"fmt"
	"strings"
)

// This file adds PostgreSQL-flavoured schema-object helpers that ActiveRecord
// exposes for that adapter: enable_extension / disable_extension, create_enum /
// drop_enum, and ALTER TYPE ... ADD VALUE. The statements are standard
// PostgreSQL DDL. Every helper returns a SQL string for the bound [Dialect] and
// touches no database.

// EnableExtension builds a CREATE EXTENSION IF NOT EXISTS statement, loading a
// PostgreSQL extension such as "pgcrypto" or "uuid-ossp". The extension name is
// double-quoted so hyphenated names are handled correctly.
func (s *Schema) EnableExtension(name string) string {
	return `CREATE EXTENSION IF NOT EXISTS "` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// DisableExtension builds a DROP EXTENSION IF EXISTS statement, unloading a
// PostgreSQL extension previously enabled with [Schema.EnableExtension].
func (s *Schema) DisableExtension(name string) string {
	return `DROP EXTENSION IF EXISTS "` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// CreateEnum builds a CREATE TYPE ... AS ENUM statement declaring a PostgreSQL
// enumerated type. The values are emitted as quoted string literals. Reference
// the type from a column with [Table.EnumType].
func (s *Schema) CreateEnum(name string, values []string) string {
	return fmt.Sprintf("CREATE TYPE %s AS ENUM (%s)",
		s.dialect.Quote(name), enumValues(values))
}

// DropEnum builds a DROP TYPE statement removing an enumerated type created with
// [Schema.CreateEnum].
func (s *Schema) DropEnum(name string) string {
	return "DROP TYPE " + s.dialect.Quote(name)
}

// enumValueOptions holds resolved modifiers for [Schema.AddEnumValue].
type enumValueOptions struct {
	before string
	after  string
}

// EnumValueOption positions a new value within an existing enumerated type.
type EnumValueOption func(*enumValueOptions)

// BeforeValue inserts the new enum value immediately before an existing value.
func BeforeValue(v string) EnumValueOption {
	return func(o *enumValueOptions) { o.before = v }
}

// AfterValue inserts the new enum value immediately after an existing value.
func AfterValue(v string) EnumValueOption {
	return func(o *enumValueOptions) { o.after = v }
}

// AddEnumValue builds an ALTER TYPE ... ADD VALUE statement adding value to a
// PostgreSQL enumerated type. By default the value is appended; use
// [BeforeValue] or [AfterValue] to position it. If both are given, [BeforeValue]
// takes precedence.
func (s *Schema) AddEnumValue(enumName, value string, opts ...EnumValueOption) string {
	var o enumValueOptions
	for _, opt := range opts {
		opt(&o)
	}
	stmt := fmt.Sprintf("ALTER TYPE %s ADD VALUE %s", s.dialect.Quote(enumName), quote(value))
	switch {
	case o.before != "":
		stmt += " BEFORE " + quote(o.before)
	case o.after != "":
		stmt += " AFTER " + quote(o.after)
	}
	return stmt
}

// package-level convenience wrappers (ANSI dialect) -------------------------

// EnableExtension builds a CREATE EXTENSION statement using the [ANSI] dialect.
func EnableExtension(name string) string { return ansiSchema.EnableExtension(name) }

// DisableExtension builds a DROP EXTENSION statement using the [ANSI] dialect.
func DisableExtension(name string) string { return ansiSchema.DisableExtension(name) }

// CreateEnum builds a CREATE TYPE ... AS ENUM statement using the [ANSI] dialect.
func CreateEnum(name string, values []string) string { return ansiSchema.CreateEnum(name, values) }

// DropEnum builds a DROP TYPE statement using the [ANSI] dialect.
func DropEnum(name string) string { return ansiSchema.DropEnum(name) }

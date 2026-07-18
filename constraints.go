package migrate

import (
	"fmt"
	"strings"
)

// This file adds table-level constraint helpers toward ActiveRecord parity:
// CHECK constraints (Rails 6.1's add_check_constraint / remove_check_constraint)
// and named UNIQUE constraints (Rails 7's add_unique_constraint /
// remove_unique_constraint). Like the rest of the DSL, every helper returns a
// SQL string for the bound [Dialect] and touches no database.

// constraintOptions holds resolved modifiers for the constraint helpers.
type constraintOptions struct {
	name       string
	deferrable bool
}

// ConstraintOption modifies a table constraint statement produced by
// [Schema.AddCheckConstraint], [Schema.AddUniqueConstraint], and their removers.
type ConstraintOption func(*constraintOptions)

// ConstraintName overrides the generated constraint name. When omitted, a
// deterministic name is derived from the table and the constraint body so that
// the matching "remove" helper reproduces the same name.
func ConstraintName(name string) ConstraintOption {
	return func(o *constraintOptions) { o.name = name }
}

// Deferrable renders the constraint as DEFERRABLE INITIALLY DEFERRED. It applies
// to UNIQUE constraints (PostgreSQL); dialects and constraint kinds that do not
// support deferral ignore it.
func Deferrable() ConstraintOption {
	return func(o *constraintOptions) { o.deferrable = true }
}

// resolveConstraint applies the options and, when no explicit name was given,
// fills in a deterministic default derived from prefix, table, and body.
func ddlResolveConstraint(prefix, table, body string, opts []ConstraintOption) constraintOptions {
	var o constraintOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.name == "" {
		frag := strings.Trim(sanitizeIdent(body), "_")
		o.name = prefix + "_" + table + "_" + frag
	}
	return o
}

// AddCheckConstraint builds an ALTER TABLE ... ADD CONSTRAINT ... CHECK (expr)
// statement. The expression is emitted verbatim. When [ConstraintName] is not
// supplied, a deterministic name ("chk_<table>_<sanitized-expr>") is generated
// so [Schema.RemoveCheckConstraint] with the same expression targets it.
func (s *Schema) AddCheckConstraint(table, expression string, opts ...ConstraintOption) string {
	o := ddlResolveConstraint("chk", table, expression, opts)
	return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s)",
		s.dialect.Quote(table), s.dialect.Quote(o.name), expression)
}

// RemoveCheckConstraint builds an ALTER TABLE ... DROP CONSTRAINT statement
// removing a CHECK constraint. The same table and expression (or [ConstraintName])
// that created the constraint reproduce its name here.
func (s *Schema) RemoveCheckConstraint(table, expression string, opts ...ConstraintOption) string {
	o := ddlResolveConstraint("chk", table, expression, opts)
	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s",
		s.dialect.Quote(table), s.dialect.Quote(o.name))
}

// AddUniqueConstraint builds an ALTER TABLE ... ADD CONSTRAINT ... UNIQUE (cols)
// statement. Unlike a unique index, a unique constraint can be referenced by a
// foreign key and marked [Deferrable]. When [ConstraintName] is not supplied a
// deterministic name ("uniq_<table>_<cols>") is generated.
func (s *Schema) AddUniqueConstraint(table string, columns []string, opts ...ConstraintOption) string {
	body := strings.Join(sanitizeAll(columns), "_")
	o := ddlResolveConstraint("uniq", table, body, opts)
	rendered := make([]string, len(columns))
	for i, c := range columns {
		rendered[i] = s.dialect.Quote(c)
	}
	stmt := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s)",
		s.dialect.Quote(table), s.dialect.Quote(o.name), strings.Join(rendered, ", "))
	if o.deferrable {
		stmt += " DEFERRABLE INITIALLY DEFERRED"
	}
	return stmt
}

// RemoveUniqueConstraint builds an ALTER TABLE ... DROP CONSTRAINT statement
// removing a unique constraint added with [Schema.AddUniqueConstraint]. The same
// columns (or [ConstraintName]) reproduce the constraint name.
func (s *Schema) RemoveUniqueConstraint(table string, columns []string, opts ...ConstraintOption) string {
	body := strings.Join(sanitizeAll(columns), "_")
	o := ddlResolveConstraint("uniq", table, body, opts)
	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s",
		s.dialect.Quote(table), s.dialect.Quote(o.name))
}

// package-level convenience wrappers (ANSI dialect) -------------------------

// AddCheckConstraint builds a CHECK constraint using the [ANSI] dialect.
func AddCheckConstraint(table, expression string, opts ...ConstraintOption) string {
	return ansiSchema.AddCheckConstraint(table, expression, opts...)
}

// RemoveCheckConstraint builds a CHECK constraint removal using the [ANSI] dialect.
func RemoveCheckConstraint(table, expression string, opts ...ConstraintOption) string {
	return ansiSchema.RemoveCheckConstraint(table, expression, opts...)
}

// AddUniqueConstraint builds a UNIQUE constraint using the [ANSI] dialect.
func AddUniqueConstraint(table string, columns []string, opts ...ConstraintOption) string {
	return ansiSchema.AddUniqueConstraint(table, columns, opts...)
}

// RemoveUniqueConstraint builds a UNIQUE constraint removal using the [ANSI] dialect.
func RemoveUniqueConstraint(table string, columns []string, opts ...ConstraintOption) string {
	return ansiSchema.RemoveUniqueConstraint(table, columns, opts...)
}

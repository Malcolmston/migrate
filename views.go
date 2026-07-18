package migrate

import (
	"fmt"
	"strings"
)

// This file adds SQL view helpers. Database views are a common companion to
// migrations (ActiveRecord projects reach for them via extensions such as
// Scenic); the helpers here emit standard CREATE VIEW / DROP VIEW DDL plus
// PostgreSQL materialized-view support. Every helper returns a SQL string for
// the bound [Dialect] and touches no database.

// viewOptions holds resolved modifiers for the view helpers.
type viewOptions struct {
	orReplace    bool
	materialized bool
	ifExists     bool
}

// ViewOption modifies a view statement produced by [Schema.CreateView] and
// [Schema.DropView].
type ViewOption func(*viewOptions)

// OrReplace renders CREATE OR REPLACE VIEW, replacing an existing view in place.
// It is ignored for materialized views, which cannot be replaced this way.
func OrReplace() ViewOption { return func(o *viewOptions) { o.orReplace = true } }

// Materialized renders a MATERIALIZED VIEW (PostgreSQL), whose result set is
// stored on disk and refreshed with [Schema.RefreshMaterializedView].
func Materialized() ViewOption { return func(o *viewOptions) { o.materialized = true } }

// ViewIfExists adds IF EXISTS to a [Schema.DropView] statement.
func ViewIfExists() ViewOption { return func(o *viewOptions) { o.ifExists = true } }

// CreateView builds a CREATE VIEW statement whose body is the given SELECT
// query, emitted verbatim. Combine with [OrReplace] to replace an existing view
// or [Materialized] for a PostgreSQL materialized view.
func (s *Schema) CreateView(name, query string, opts ...ViewOption) string {
	var o viewOptions
	for _, opt := range opts {
		opt(&o)
	}
	head := "CREATE "
	if o.orReplace && !o.materialized {
		head = "CREATE OR REPLACE "
	}
	if o.materialized {
		head += "MATERIALIZED VIEW "
	} else {
		head += "VIEW "
	}
	return fmt.Sprintf("%s%s AS %s", head, s.dialect.Quote(name), strings.TrimSpace(query))
}

// DropView builds a DROP VIEW statement. Pass [Materialized] to drop a
// materialized view and [ViewIfExists] to add IF EXISTS.
func (s *Schema) DropView(name string, opts ...ViewOption) string {
	var o viewOptions
	for _, opt := range opts {
		opt(&o)
	}
	head := "DROP "
	if o.materialized {
		head += "MATERIALIZED VIEW "
	} else {
		head += "VIEW "
	}
	if o.ifExists {
		head += "IF EXISTS "
	}
	return head + s.dialect.Quote(name)
}

// RefreshMaterializedView builds a REFRESH MATERIALIZED VIEW statement
// (PostgreSQL), recomputing a materialized view's stored result set.
func (s *Schema) RefreshMaterializedView(name string) string {
	return "REFRESH MATERIALIZED VIEW " + s.dialect.Quote(name)
}

// package-level convenience wrappers (ANSI dialect) -------------------------

// CreateView builds a CREATE VIEW statement using the [ANSI] dialect.
func CreateView(name, query string, opts ...ViewOption) string {
	return ansiSchema.CreateView(name, query, opts...)
}

// DropView builds a DROP VIEW statement using the [ANSI] dialect.
func DropView(name string, opts ...ViewOption) string {
	return ansiSchema.DropView(name, opts...)
}

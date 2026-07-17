package migrate

import (
	"fmt"
	"strings"
)

// indexOptions holds resolved [Schema.AddIndex] modifiers.
type indexOptions struct {
	unique bool
	name   string
	where  string
	using  string
}

// IndexOption modifies a CREATE INDEX statement.
type IndexOption func(*indexOptions)

// UniqueIndex makes the index UNIQUE.
func UniqueIndex() IndexOption { return func(o *indexOptions) { o.unique = true } }

// IndexName overrides the generated index name.
func IndexName(name string) IndexOption { return func(o *indexOptions) { o.name = name } }

// Where adds a WHERE clause, producing a partial index. The condition is
// emitted verbatim.
func Where(condition string) IndexOption { return func(o *indexOptions) { o.where = condition } }

// Using selects the index method (e.g. "gin", "gist", "hash", "btree"),
// emitted as "USING <method>".
func Using(method string) IndexOption { return func(o *indexOptions) { o.using = method } }

// AddIndex builds a CREATE INDEX statement over the given columns. Columns that
// are bare identifiers are quoted for the dialect; anything else (such as a
// functional expression "lower(email)") is emitted verbatim, enabling
// expression indexes. When no name is supplied the ActiveRecord-style
// "index_<table>_on_<cols>" convention is used.
//
// Options compose: [UniqueIndex] for uniqueness, [Using] to pick an index
// method, and [Where] for a partial index.
func (s *Schema) AddIndex(table string, columns []string, opts ...IndexOption) string {
	var o indexOptions
	for _, opt := range opts {
		opt(&o)
	}
	name := o.name
	if name == "" {
		name = "index_" + table + "_on_" + strings.Join(sanitizeAll(columns), "_")
	}

	rendered := make([]string, len(columns))
	for i, c := range columns {
		rendered[i] = s.dialect.Quote(c)
	}

	kw := "CREATE INDEX"
	if o.unique {
		kw = "CREATE UNIQUE INDEX"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s ON %s", kw, s.dialect.Quote(name), s.dialect.Quote(table))
	if o.using != "" {
		b.WriteString(" USING ")
		b.WriteString(o.using)
	}
	fmt.Fprintf(&b, " (%s)", strings.Join(rendered, ", "))
	if o.where != "" {
		b.WriteString(" WHERE ")
		b.WriteString(o.where)
	}
	return b.String()
}

// DropIndex builds a DROP INDEX statement.
func (s *Schema) DropIndex(name string) string {
	return "DROP INDEX " + s.dialect.Quote(name)
}

// sanitizeAll reduces a set of index column expressions to identifier fragments
// suitable for a generated index name.
func sanitizeAll(cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = sanitizeIdent(c)
	}
	return out
}

// sanitizeIdent keeps identifier characters and replaces everything else with
// underscores, so an expression like "lower(email)" yields "lower_email_".
func sanitizeIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

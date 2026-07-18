package migrate

import (
	"fmt"
	"strconv"
	"strings"
)

// This file adds miscellaneous table- and object-level DDL helpers: TRUNCATE,
// object comments (COMMENT ON), and sequences. Every helper returns a SQL string
// for the bound [Dialect] and touches no database.

// TruncateTable builds a TRUNCATE TABLE statement emptying one or more tables.
// At least one table name must be given; multiple names are comma-separated.
func (s *Schema) TruncateTable(names ...string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = s.dialect.Quote(n)
	}
	return "TRUNCATE TABLE " + strings.Join(quoted, ", ")
}

// SetTableComment builds a COMMENT ON TABLE statement attaching a comment to a
// table, mirroring ActiveRecord's table comment option. The comment is emitted
// as a quoted string literal.
func (s *Schema) SetTableComment(table, comment string) string {
	return fmt.Sprintf("COMMENT ON TABLE %s IS %s", s.dialect.Quote(table), quote(comment))
}

// SetColumnComment builds a COMMENT ON COLUMN statement attaching a comment to a
// column. The comment is emitted as a quoted string literal.
func (s *Schema) SetColumnComment(table, column, comment string) string {
	return fmt.Sprintf("COMMENT ON COLUMN %s.%s IS %s",
		s.dialect.Quote(table), s.dialect.Quote(column), quote(comment))
}

// sequenceOptions holds resolved modifiers for [Schema.CreateSequence].
type sequenceOptions struct {
	start        int64
	hasStart     bool
	increment    int64
	hasIncrement bool
}

// SequenceOption modifies a [Schema.CreateSequence] statement.
type SequenceOption func(*sequenceOptions)

// SequenceStart sets the sequence's initial value (START WITH n).
func SequenceStart(n int64) SequenceOption {
	return func(o *sequenceOptions) { o.start = n; o.hasStart = true }
}

// SequenceIncrement sets the sequence's step (INCREMENT BY n).
func SequenceIncrement(n int64) SequenceOption {
	return func(o *sequenceOptions) { o.increment = n; o.hasIncrement = true }
}

// CreateSequence builds a CREATE SEQUENCE statement. Use [SequenceStart] and
// [SequenceIncrement] to set the initial value and step; omitted clauses fall
// back to the database defaults.
func (s *Schema) CreateSequence(name string, opts ...SequenceOption) string {
	var o sequenceOptions
	for _, opt := range opts {
		opt(&o)
	}
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE ")
	b.WriteString(s.dialect.Quote(name))
	if o.hasIncrement {
		b.WriteString(" INCREMENT BY ")
		b.WriteString(strconv.FormatInt(o.increment, 10))
	}
	if o.hasStart {
		b.WriteString(" START WITH ")
		b.WriteString(strconv.FormatInt(o.start, 10))
	}
	return b.String()
}

// DropSequence builds a DROP SEQUENCE statement removing a sequence created with
// [Schema.CreateSequence].
func (s *Schema) DropSequence(name string) string {
	return "DROP SEQUENCE " + s.dialect.Quote(name)
}

// package-level convenience wrappers (ANSI dialect) -------------------------

// TruncateTable builds a TRUNCATE TABLE statement using the [ANSI] dialect.
func TruncateTable(names ...string) string { return ansiSchema.TruncateTable(names...) }

// SetTableComment builds a COMMENT ON TABLE statement using the [ANSI] dialect.
func SetTableComment(table, comment string) string {
	return ansiSchema.SetTableComment(table, comment)
}

// SetColumnComment builds a COMMENT ON COLUMN statement using the [ANSI] dialect.
func SetColumnComment(table, column, comment string) string {
	return ansiSchema.SetColumnComment(table, column, comment)
}

// CreateSequence builds a CREATE SEQUENCE statement using the [ANSI] dialect.
func CreateSequence(name string, opts ...SequenceOption) string {
	return ansiSchema.CreateSequence(name, opts...)
}

// DropSequence builds a DROP SEQUENCE statement using the [ANSI] dialect.
func DropSequence(name string) string { return ansiSchema.DropSequence(name) }

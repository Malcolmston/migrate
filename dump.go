package migrate

import (
	"context"
	"fmt"
	"strings"
)

// SchemaDump accumulates DDL statements into a single, reconstructable schema
// script stamped with a schema version, analogous to ActiveRecord's schema.rb.
// Running the emitted script against an empty database recreates the schema.
//
// A dump is dialect-aware: its builder methods render statements for the bound
// [Dialect]. Statements are emitted in the order they are recorded.
type SchemaDump struct {
	schema  *Schema
	version uint64
	stmts   []string
}

// NewSchemaDump returns a [SchemaDump] that renders statements for d (nil means
// [ANSI]) and records the given schema version in the header.
func NewSchemaDump(d Dialect, version uint64) *SchemaDump {
	return &SchemaDump{schema: NewSchema(d), version: version}
}

// Version returns the schema version stamped on the dump.
func (s *SchemaDump) Version() uint64 { return s.version }

// SetVersion updates the stamped schema version and returns the dump for
// chaining.
func (s *SchemaDump) SetVersion(v uint64) *SchemaDump { s.version = v; return s }

// Add appends one or more already-rendered statements verbatim.
func (s *SchemaDump) Add(stmts ...string) *SchemaDump {
	s.stmts = append(s.stmts, stmts...)
	return s
}

// CreateTable records a CREATE TABLE statement.
func (s *SchemaDump) CreateTable(name string, build func(t *Table), opts ...TableOption) *SchemaDump {
	return s.Add(s.schema.CreateTable(name, build, opts...))
}

// AddIndex records a CREATE INDEX statement.
func (s *SchemaDump) AddIndex(table string, columns []string, opts ...IndexOption) *SchemaDump {
	return s.Add(s.schema.AddIndex(table, columns, opts...))
}

// AddForeignKey records an ADD CONSTRAINT foreign key statement.
func (s *SchemaDump) AddForeignKey(fromTable, toTable string, opts ...ForeignKeyOption) *SchemaDump {
	return s.Add(s.schema.AddForeignKey(fromTable, toTable, opts...))
}

// Statements returns the recorded statements in order.
func (s *SchemaDump) Statements() []string {
	return append([]string(nil), s.stmts...)
}

// String renders the full dump: a header comment carrying the dialect and
// schema version, followed by every recorded statement terminated by ";". The
// output is valid SQL that reconstructs the schema.
func (s *SchemaDump) String() string {
	var b strings.Builder
	b.WriteString("-- migrate schema dump. Do not edit by hand.\n")
	fmt.Fprintf(&b, "-- dialect: %s\n", s.schema.dialect.Name())
	fmt.Fprintf(&b, "-- version: %d\n", s.version)
	b.WriteString("\n")
	for _, stmt := range s.stmts {
		b.WriteString(stmt)
		b.WriteString(";\n")
	}
	return b.String()
}

// Version returns the highest applied migration version recorded in the
// bookkeeping table, or 0 when nothing has been applied. It is useful for
// stamping a [SchemaDump] with the schema's current version.
func (mg *Migrator) Version(ctx context.Context) (uint64, error) {
	if err := mg.EnsureSchemaTable(ctx); err != nil {
		return 0, err
	}
	applied, err := mg.appliedSet(ctx)
	if err != nil {
		return 0, err
	}
	var max uint64
	for v := range applied {
		if v > max {
			max = v
		}
	}
	return max, nil
}

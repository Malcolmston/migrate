package migrate

import (
	"fmt"
	"strings"
)

// ReferentialAction is an ON DELETE / ON UPDATE referential action.
type ReferentialAction string

// The standard referential actions.
const (
	// NoAction rejects the change if it would violate the constraint (default).
	NoAction ReferentialAction = "NO ACTION"
	// Restrict is like NoAction but checked immediately.
	Restrict ReferentialAction = "RESTRICT"
	// Cascade propagates the delete/update to referencing rows.
	Cascade ReferentialAction = "CASCADE"
	// SetNull sets referencing columns to NULL.
	SetNull ReferentialAction = "SET NULL"
	// SetDefault sets referencing columns to their default value.
	SetDefault ReferentialAction = "SET DEFAULT"
)

// fkOptions holds resolved foreign-key modifiers.
type fkOptions struct {
	column     string
	primaryKey string
	name       string
	onDelete   ReferentialAction
	onUpdate   ReferentialAction
}

// ForeignKeyOption modifies an [Schema.AddForeignKey] statement.
type ForeignKeyOption func(*fkOptions)

// FKColumn overrides the referencing column (default "<singular-to>_id").
func FKColumn(name string) ForeignKeyOption { return func(o *fkOptions) { o.column = name } }

// FKPrimaryKey overrides the referenced column on the target table (default
// "id").
func FKPrimaryKey(name string) ForeignKeyOption { return func(o *fkOptions) { o.primaryKey = name } }

// FKName overrides the generated constraint name.
func FKName(name string) ForeignKeyOption { return func(o *fkOptions) { o.name = name } }

// OnDelete sets the ON DELETE referential action.
func OnDelete(a ReferentialAction) ForeignKeyOption { return func(o *fkOptions) { o.onDelete = a } }

// OnUpdate sets the ON UPDATE referential action.
func OnUpdate(a ReferentialAction) ForeignKeyOption { return func(o *fkOptions) { o.onUpdate = a } }

// resolveFK fills in the conventional defaults for a foreign key from fromTable
// to toTable.
func resolveFK(fromTable, toTable string, opts []ForeignKeyOption) fkOptions {
	o := fkOptions{
		column:     singularize(toTable) + "_id",
		primaryKey: "id",
	}
	for _, opt := range opts {
		opt(&o)
	}
	if o.name == "" {
		o.name = "fk_" + fromTable + "_" + o.column
	}
	return o
}

// AddForeignKey builds an ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY
// statement linking fromTable to toTable. By default the referencing column is
// "<singular-toTable>_id" and the referenced column is "id"; override with
// [FKColumn] and [FKPrimaryKey]. Referential actions are added with [OnDelete]
// and [OnUpdate].
func (s *Schema) AddForeignKey(fromTable, toTable string, opts ...ForeignKeyOption) string {
	o := resolveFK(fromTable, toTable, opts)
	var b strings.Builder
	fmt.Fprintf(&b, "ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
		s.dialect.Quote(fromTable), s.dialect.Quote(o.name), s.dialect.Quote(o.column),
		s.dialect.Quote(toTable), s.dialect.Quote(o.primaryKey))
	if o.onDelete != "" {
		b.WriteString(" ON DELETE ")
		b.WriteString(string(o.onDelete))
	}
	if o.onUpdate != "" {
		b.WriteString(" ON UPDATE ")
		b.WriteString(string(o.onUpdate))
	}
	return b.String()
}

// RemoveForeignKey builds an ALTER TABLE ... DROP CONSTRAINT statement removing
// the foreign key from fromTable to toTable. The same options that named the
// constraint in [Schema.AddForeignKey] reproduce the name here; pass [FKName]
// to target an explicitly named constraint.
func (s *Schema) RemoveForeignKey(fromTable, toTable string, opts ...ForeignKeyOption) string {
	o := resolveFK(fromTable, toTable, opts)
	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s",
		s.dialect.Quote(fromTable), s.dialect.Quote(o.name))
}

// AddReference builds statement(s) adding a "<name>_id" foreign-key column to an
// existing table, mirroring ActiveRecord's add_reference. By default it only
// adds the column; pass [WithForeignKey] to also emit an ADD CONSTRAINT
// foreign key, and [ReferenceIndex] to add an index on the new column.
func (s *Schema) AddReference(table, name string, opts ...ReferenceOption) string {
	ro := referenceOptions{column: name + "_id", refTable: pluralize(name), refColumn: "id"}
	for _, opt := range opts {
		opt(&ro)
	}
	var colOpts []ColumnOption
	if ro.notNull {
		colOpts = append(colOpts, NotNull())
	}
	stmts := []string{s.AddColumn(table, ro.column, "BIGINT", colOpts...)}
	if ro.index {
		stmts = append(stmts, s.AddIndex(table, []string{ro.column}))
	}
	if ro.foreignKey {
		stmts = append(stmts, s.AddForeignKey(table, ro.refTable,
			FKColumn(ro.column), FKPrimaryKey(ro.refColumn)))
	}
	return strings.Join(stmts, ";\n")
}

// RemoveReference builds statement(s) dropping a reference column previously
// added with [Schema.AddReference].
func (s *Schema) RemoveReference(table, name string, opts ...ReferenceOption) string {
	ro := referenceOptions{column: name + "_id", refTable: pluralize(name), refColumn: "id"}
	for _, opt := range opts {
		opt(&ro)
	}
	return s.DropColumn(table, ro.column)
}

// singularize is the naive inverse of [pluralize]: it strips a single trailing
// "s". It is deliberately simple; override column names explicitly when wrong.
func singularize(s string) string {
	if strings.HasSuffix(s, "s") && len(s) > 1 {
		return s[:len(s)-1]
	}
	return s
}

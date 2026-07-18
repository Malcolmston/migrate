package migrate

// This file extends [ChangeRecorder] with reversible variants of the newer
// schema helpers (check constraints, join tables, index rename, views, and
// extensions), so they participate in automatically-inverted [Change] migrations.

// AddCheckConstraint records adding a CHECK constraint; its inverse removes it.
// The same table and expression reproduce the constraint name on rollback.
func (r *ChangeRecorder) AddCheckConstraint(table, expression string, opts ...ConstraintOption) {
	r.record(
		[]string{r.schema.AddCheckConstraint(table, expression, opts...)},
		[]string{r.schema.RemoveCheckConstraint(table, expression, opts...)},
	)
}

// AddUniqueConstraint records adding a UNIQUE constraint; its inverse removes it.
func (r *ChangeRecorder) AddUniqueConstraint(table string, columns []string, opts ...ConstraintOption) {
	r.record(
		[]string{r.schema.AddUniqueConstraint(table, columns, opts...)},
		[]string{r.schema.RemoveUniqueConstraint(table, columns, opts...)},
	)
}

// CreateJoinTable records creating a many-to-many join table; its inverse drops
// it.
func (r *ChangeRecorder) CreateJoinTable(table1, table2 string, build func(t *Table), opts ...TableOption) {
	r.record(
		[]string{r.schema.CreateJoinTable(table1, table2, build, opts...)},
		[]string{r.schema.DropJoinTable(table1, table2)},
	)
}

// RenameIndex records renaming an index; its inverse renames back.
func (r *ChangeRecorder) RenameIndex(table, from, to string) {
	r.record(
		[]string{r.schema.RenameIndex(table, from, to)},
		[]string{r.schema.RenameIndex(table, to, from)},
	)
}

// CreateView records creating a view; its inverse drops it. When the view is
// [Materialized], the drop targets a materialized view too.
func (r *ChangeRecorder) CreateView(name, query string, opts ...ViewOption) {
	var o viewOptions
	for _, opt := range opts {
		opt(&o)
	}
	var dropOpts []ViewOption
	if o.materialized {
		dropOpts = append(dropOpts, Materialized())
	}
	r.record(
		[]string{r.schema.CreateView(name, query, opts...)},
		[]string{r.schema.DropView(name, dropOpts...)},
	)
}

// EnableExtension records enabling a PostgreSQL extension; its inverse disables
// it.
func (r *ChangeRecorder) EnableExtension(name string) {
	r.record(
		[]string{r.schema.EnableExtension(name)},
		[]string{r.schema.DisableExtension(name)},
	)
}

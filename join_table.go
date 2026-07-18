package migrate

// This file adds many-to-many join-table helpers, mirroring ActiveRecord's
// create_join_table / drop_join_table used for has_and_belongs_to_many
// associations. Every helper returns a SQL string for the bound [Dialect] and
// touches no database.

// JoinTableName returns the conventional join-table name for two tables: the two
// names sorted lexicographically and joined with an underscore, matching
// ActiveRecord's default (e.g. JoinTableName("parts", "assemblies") ==
// "assemblies_parts").
func JoinTableName(a, b string) string {
	if a <= b {
		return a + "_" + b
	}
	return b + "_" + a
}

// CreateJoinTable builds a CREATE TABLE statement for a many-to-many join table
// linking table1 and table2. The table has no surrogate id primary key; it holds
// two NOT NULL BIGINT foreign-key columns named "<singular>_id" for each side.
// The optional build callback adds further columns (for example a Timestamps or
// a payload column). The generated table name follows [JoinTableName].
func (s *Schema) CreateJoinTable(table1, table2 string, build func(t *Table), opts ...TableOption) string {
	name := JoinTableName(table1, table2)
	col1 := singularize(table1) + "_id"
	col2 := singularize(table2) + "_id"
	allOpts := append([]TableOption{WithoutID()}, opts...)
	return s.CreateTable(name, func(t *Table) {
		t.BigInteger(col1, NotNull())
		t.BigInteger(col2, NotNull())
		if build != nil {
			build(t)
		}
	}, allOpts...)
}

// DropJoinTable builds a DROP TABLE statement for the join table linking table1
// and table2 (see [JoinTableName]).
func (s *Schema) DropJoinTable(table1, table2 string) string {
	return s.DropTable(JoinTableName(table1, table2))
}

// package-level convenience wrappers (ANSI dialect) -------------------------

// CreateJoinTable builds a join-table creation using the [ANSI] dialect.
func CreateJoinTable(table1, table2 string, build func(t *Table), opts ...TableOption) string {
	return ansiSchema.CreateJoinTable(table1, table2, build, opts...)
}

// DropJoinTable builds a join-table drop using the [ANSI] dialect.
func DropJoinTable(table1, table2 string) string {
	return ansiSchema.DropJoinTable(table1, table2)
}

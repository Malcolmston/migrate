package migrate

// This file extends [SchemaDump] with recorders for the newer schema objects, so
// a version-stamped dump can reconstruct views, enumerated types, extensions,
// join tables, and check constraints, not just tables and indexes.

// CreateView records a CREATE VIEW statement.
func (s *SchemaDump) CreateView(name, query string, opts ...ViewOption) *SchemaDump {
	return s.Add(s.schema.CreateView(name, query, opts...))
}

// AddCheckConstraint records an ADD CONSTRAINT ... CHECK statement.
func (s *SchemaDump) AddCheckConstraint(table, expression string, opts ...ConstraintOption) *SchemaDump {
	return s.Add(s.schema.AddCheckConstraint(table, expression, opts...))
}

// AddUniqueConstraint records an ADD CONSTRAINT ... UNIQUE statement.
func (s *SchemaDump) AddUniqueConstraint(table string, columns []string, opts ...ConstraintOption) *SchemaDump {
	return s.Add(s.schema.AddUniqueConstraint(table, columns, opts...))
}

// EnableExtension records a CREATE EXTENSION statement.
func (s *SchemaDump) EnableExtension(name string) *SchemaDump {
	return s.Add(s.schema.EnableExtension(name))
}

// CreateEnum records a CREATE TYPE ... AS ENUM statement.
func (s *SchemaDump) CreateEnum(name string, values []string) *SchemaDump {
	return s.Add(s.schema.CreateEnum(name, values))
}

// CreateJoinTable records a join-table CREATE TABLE statement.
func (s *SchemaDump) CreateJoinTable(table1, table2 string, build func(t *Table), opts ...TableOption) *SchemaDump {
	return s.Add(s.schema.CreateJoinTable(table1, table2, build, opts...))
}

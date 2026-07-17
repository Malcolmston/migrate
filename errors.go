package migrate

import "errors"

var (
	// ErrDuplicateVersion is returned by [Migrator.Register] and the loaders
	// when two migrations share the same Version.
	ErrDuplicateVersion = errors.New("migrate: duplicate migration version")

	// ErrMissingMigration is returned when the migrator is asked to roll back a
	// version that has been applied to the database but has no registered
	// migration providing a Down direction.
	ErrMissingMigration = errors.New("migrate: applied version has no registered migration")

	// ErrInvalidTableName is returned by [New] / [WithTable] when the bookkeeping
	// table name is not a safe SQL identifier.
	ErrInvalidTableName = errors.New("migrate: invalid schema table name")

	// ErrInvalidMigration is returned when a migration is missing a version or a
	// usable Up direction.
	ErrInvalidMigration = errors.New("migrate: invalid migration")
)

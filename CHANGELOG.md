# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [0.2.0] - 2026-07-17

Major expansion of the schema DSL toward ActiveRecord::Migration parity. Fully
backward compatible: every v0.1.0 helper keeps its exact ANSI output.

### Added

- **Full column vocabulary** on `*Table`: `Decimal(precision, scale)`, `Time`,
  `Binary`, `JSON`, `JSONB`, `UUID`, `Enum` / `EnumType`, and `Array()` columns,
  plus `Timestamp` precision (`Precision`) and time-zone (`WithTimezone`)
  modifiers.
- **Per-dialect SQL** via the `Dialect` interface and the `ANSI`, `Postgres`,
  `MySQL`, and `SQLite` implementations, selectable with `DialectByName`. Each
  handles identifier quoting, bind-parameter style (`Placeholder`),
  auto-increment primary keys, and abstract → concrete column-type mapping. Bind
  one with `NewSchema` for dialect-aware DDL.
- **`ChangeTable`** bulk-alter builder (`*AlterTable`) with `Column`, typed
  helpers, `Remove`, `Rename`, `Change`, `Index`, `RemoveIndex`, `References`,
  and `Timestamps`.
- **`AddReference` / `RemoveReference`** (with `ReferenceIndex`),
  **`ChangeColumn`**, **`AddTimestamps` / `RemoveTimestamps`**.
- **Foreign keys**: `AddForeignKey` / `RemoveForeignKey` with `OnDelete` /
  `OnUpdate` referential actions (`Cascade`, `Restrict`, `SetNull`,
  `SetDefault`, `NoAction`) and `FKColumn` / `FKPrimaryKey` / `FKName` overrides.
- **Advanced indexes**: `UniqueIndex`, partial indexes (`Where`),
  expression/functional indexes, and index methods (`Using`).
- **Reversible migrations**: `Change` / `ChangeWith` with a `ChangeRecorder`
  that auto-generates the inverse, and irreversible-operation detection
  surfacing `ErrIrreversibleMigration`.
- **Schema dump**: `SchemaDump` collects DDL into a version-stamped,
  reconstructable script; `Migrator.Version` reports the current schema version.
- **Seeds**: `Seeder` for idempotent, name-tracked data loading (`Run`,
  `RunSQL`, `Applied`), plus `Execute` / `ExecuteAll` raw-SQL helpers.
- New sentinel errors `ErrIrreversibleMigration` and `ErrUnknownDialect`.

### Changed

- The column type system now resolves abstract types through a `Dialect`; the
  package-level helpers render with `ANSI` and preserve prior output byte for
  byte.

## [0.1.0]

- Initial release: versioned reversible migrations, `Migrate` / `Up` / `Down` /
  `Rollback` / `MigrateTo` / `Redo` / `Status`, directory and `io/fs` loaders,
  and the initial ANSI schema DSL.

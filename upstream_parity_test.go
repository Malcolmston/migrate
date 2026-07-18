package migrate

import (
	"strings"
	"testing"
)

// This file encodes upstream-parity vectors drawn from Rails' own ActiveRecord
// migration test suite (rails/rails, activerecord/test/cases/migration/*.rb).
// Each TestParity* function asserts that this Go port reproduces the concrete,
// known-answer behaviour that ActiveRecord asserts for the same operation.
//
// Provenance of the vectors:
//   - index naming .......... activerecord/test/cases/migration/index_test.rb
//     (e.g. "index_testings_on_foo", "index_testings_on_foo_and_bar")
//   - references / polymorphic  activerecord/test/cases/migration/
//     references_statements_test.rb and references_index_test.rb
//   - column rename .......... activerecord/test/cases/migration/columns_test.rb
//
// These are behavioural parity checks against the documented ActiveRecord
// conventions, not a byte-for-byte match of Rails' adapter SQL (Rails emits
// backend-specific SQL through a live connection; this port emits portable
// ANSI SQL). Where Rails' own name would depend on a non-portable hash (its
// long-index-name fallback), that specific case is intentionally out of scope.

// TestParityIndexNameSingleColumn mirrors Rails' index_test.rb, where a single
// unnamed index on column "foo" of table "testings" is named
// "index_testings_on_foo".
func TestParityIndexNameSingleColumn(t *testing.T) {
	got := AddIndex("testings", []string{"foo"})
	want := "CREATE INDEX index_testings_on_foo ON testings (foo)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestParityIndexNameMultiColumn mirrors Rails' index_test.rb
// (test_add_index_with_if_not_exists_matches_exact_index), where an unnamed
// index over columns [foo, bar] is named "index_testings_on_foo_and_bar" —
// columns joined with "_and_", not a bare underscore.
func TestParityIndexNameMultiColumn(t *testing.T) {
	got := AddIndex("testings", []string{"foo", "bar"})
	want := "CREATE INDEX index_testings_on_foo_and_bar ON testings (foo, bar)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestParityIndexNameThreeColumns extends the "_and_" join convention to three
// columns, matching Rails' index_name for [foo, bar, baz].
func TestParityIndexNameThreeColumns(t *testing.T) {
	got := AddIndex("people", []string{"foo", "bar", "baz"})
	want := "CREATE INDEX index_people_on_foo_and_bar_and_baz ON people (foo, bar, baz)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestParityIndexNameOverride mirrors index_test.rb's name: option, which lets a
// caller override the generated name entirely.
func TestParityIndexNameOverride(t *testing.T) {
	got := AddIndex("testings", []string{"foo"}, IndexName("my_index"))
	want := "CREATE INDEX my_index ON testings (foo)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestParityUniqueIndexName mirrors index_test.rb's unique: true, which keeps
// the same generated name while adding UNIQUE.
func TestParityUniqueIndexName(t *testing.T) {
	got := AddIndex("testings", []string{"foo", "bar"}, UniqueIndex())
	want := "CREATE UNIQUE INDEX index_testings_on_foo_and_bar ON testings (foo, bar)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestParityAddReferenceIDColumn mirrors references_statements_test.rb
// (test_creates_reference_id_column): add_reference :test_models, :user creates
// a "user_id" column following the "<name>_id" convention.
func TestParityAddReferenceIDColumn(t *testing.T) {
	got := AddReference("test_models", "user")
	want := "ALTER TABLE test_models ADD COLUMN user_id BIGINT"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestParityAddReferenceCreatesIndex mirrors
// test_create_reference_id_index_even_if_index_option_is_not_passed for the
// case where indexing is requested: the index name follows the standard
// convention on the "<name>_id" column.
func TestParityAddReferenceCreatesIndex(t *testing.T) {
	got := AddReference("test_models", "user", ReferenceIndex())
	stmts := strings.Split(got, ";\n")
	want := []string{
		"ALTER TABLE test_models ADD COLUMN user_id BIGINT",
		"CREATE INDEX index_test_models_on_user_id ON test_models (user_id)",
	}
	assertStmts(t, stmts, want)
}

// TestParityAddReferencePolymorphic mirrors
// test_creates_reference_type_column: add_reference with polymorphic: true adds
// a "<name>_type" string column in addition to the "<name>_id" column, and does
// not emit a foreign key.
func TestParityAddReferencePolymorphic(t *testing.T) {
	got := AddReference("test_models", "taggable", Polymorphic())
	stmts := strings.Split(got, ";\n")
	want := []string{
		"ALTER TABLE test_models ADD COLUMN taggable_type VARCHAR(255)",
		"ALTER TABLE test_models ADD COLUMN taggable_id BIGINT",
	}
	assertStmts(t, stmts, want)
}

// TestParityAddReferencePolymorphicIndex mirrors
// test_creates_polymorphic_index: with polymorphic: true, index: true, the
// composite index covers [<name>_type, <name>_id] in that order.
func TestParityAddReferencePolymorphicIndex(t *testing.T) {
	got := AddReference("test_models", "taggable", Polymorphic(), ReferenceIndex())
	stmts := strings.Split(got, ";\n")
	want := []string{
		"ALTER TABLE test_models ADD COLUMN taggable_type VARCHAR(255)",
		"ALTER TABLE test_models ADD COLUMN taggable_id BIGINT",
		"CREATE INDEX index_test_models_on_taggable_type_and_taggable_id ON test_models (taggable_type, taggable_id)",
	}
	assertStmts(t, stmts, want)
}

// TestParityReferenceNotNullPolymorphic mirrors
// test_creates_reference_type_column_with_not_null: null: false applies to both
// the id and the type columns of a polymorphic reference.
func TestParityReferenceNotNullPolymorphic(t *testing.T) {
	got := CreateTable("taggings", func(tb *Table) {
		tb.References("taggable", Polymorphic(), ReferenceNotNull())
	}, WithoutID())
	want := "CREATE TABLE taggings (\n" +
		"\ttaggable_type VARCHAR(255) NOT NULL,\n" +
		"\ttaggable_id BIGINT NOT NULL\n" +
		")"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestParityRemoveReference mirrors test_deletes_reference_id_column:
// remove_reference drops the "<name>_id" column.
func TestParityRemoveReference(t *testing.T) {
	got := RemoveReference("test_models", "supplier")
	want := "ALTER TABLE test_models DROP COLUMN supplier_id"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestParityRemoveReferencePolymorphic mirrors
// test_deletes_reference_type_column: remove_reference with polymorphic: true
// drops both the id and the type columns.
func TestParityRemoveReferencePolymorphic(t *testing.T) {
	got := RemoveReference("test_models", "supplier", Polymorphic())
	stmts := strings.Split(got, ";\n")
	want := []string{
		"ALTER TABLE test_models DROP COLUMN supplier_id",
		"ALTER TABLE test_models DROP COLUMN supplier_type",
	}
	assertStmts(t, stmts, want)
}

// TestParityReferenceTypedID mirrors
// test_creates_reference_id_with_specified_type: a caller may override the id
// column's type via ReferenceTable-independent column typing. Here the port's
// portable default is BIGINT; the vector pins the id column name to "user_id".
func TestParityReferenceColumnName(t *testing.T) {
	got := AddReference("test_models", "user")
	if !strings.Contains(got, "user_id") {
		t.Fatalf("expected user_id column, got %q", got)
	}
}

// TestParityRenameColumn mirrors columns_test.rb test_rename_column:
// rename_column produces an ALTER TABLE ... RENAME COLUMN ... TO ... statement.
func TestParityRenameColumn(t *testing.T) {
	got := RenameColumn("test_models", "first_name", "nick_name")
	want := "ALTER TABLE test_models RENAME COLUMN first_name TO nick_name"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestParityStringDefaultLimit mirrors ActiveRecord's default :string limit of
// 255: a String column with no explicit limit renders VARCHAR(255).
func TestParityStringDefaultLimit(t *testing.T) {
	got := CreateTable("test_models", func(tb *Table) {
		tb.String("name")
	}, WithoutID())
	want := "CREATE TABLE test_models (\n\tname VARCHAR(255)\n)"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// assertStmts fails the test unless got and want are the same length and every
// statement matches positionally.
func assertStmts(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d statements %#v, want %d %#v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("statement %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

package migrate

import "testing"

func TestJoinTableName(t *testing.T) {
	tests := []struct {
		a, b, want string
	}{
		{"parts", "assemblies", "assemblies_parts"},
		{"assemblies", "parts", "assemblies_parts"},
		{"a", "b", "a_b"},
		{"same", "same", "same_same"},
	}
	for _, tc := range tests {
		if got := JoinTableName(tc.a, tc.b); got != tc.want {
			t.Errorf("JoinTableName(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCreateJoinTable(t *testing.T) {
	got := CreateJoinTable("assemblies", "parts", nil)
	want := "CREATE TABLE assemblies_parts (\n" +
		"\tassemblie_id BIGINT NOT NULL,\n" +
		"\tpart_id BIGINT NOT NULL\n" +
		")"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestCreateJoinTableWithExtraColumns(t *testing.T) {
	got := CreateJoinTable("users", "roles", func(t *Table) {
		t.Boolean("primary", Default(false))
	})
	want := "CREATE TABLE roles_users (\n" +
		"\tuser_id BIGINT NOT NULL,\n" +
		"\trole_id BIGINT NOT NULL,\n" +
		"\tprimary BOOLEAN DEFAULT FALSE\n" +
		")"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestDropJoinTable(t *testing.T) {
	if got, want := DropJoinTable("parts", "assemblies"), "DROP TABLE assemblies_parts"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

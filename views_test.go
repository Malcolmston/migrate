package migrate

import "testing"

func TestCreateView(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "plain",
			got:  CreateView("active_users", "SELECT * FROM users WHERE active"),
			want: "CREATE VIEW active_users AS SELECT * FROM users WHERE active",
		},
		{
			name: "or replace",
			got:  CreateView("active_users", "SELECT * FROM users", OrReplace()),
			want: "CREATE OR REPLACE VIEW active_users AS SELECT * FROM users",
		},
		{
			name: "materialized",
			got:  CreateView("stats", "SELECT count(*) FROM users", Materialized()),
			want: "CREATE MATERIALIZED VIEW stats AS SELECT count(*) FROM users",
		},
		{
			name: "materialized ignores or replace",
			got:  CreateView("stats", "SELECT 1", Materialized(), OrReplace()),
			want: "CREATE MATERIALIZED VIEW stats AS SELECT 1",
		},
		{
			name: "query trimmed",
			got:  CreateView("v", "  SELECT 1  "),
			want: "CREATE VIEW v AS SELECT 1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestDropView(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "plain",
			got:  DropView("active_users"),
			want: "DROP VIEW active_users",
		},
		{
			name: "materialized if exists",
			got:  DropView("stats", Materialized(), ViewIfExists()),
			want: "DROP MATERIALIZED VIEW IF EXISTS stats",
		},
		{
			name: "refresh",
			got:  NewSchema(ANSI).RefreshMaterializedView("stats"),
			want: "REFRESH MATERIALIZED VIEW stats",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

package migrate

import "testing"

func TestTruncateTable(t *testing.T) {
	if got, want := TruncateTable("users"), "TRUNCATE TABLE users"; got != want {
		t.Errorf("single: got %q, want %q", got, want)
	}
	if got, want := TruncateTable("users", "logs"), "TRUNCATE TABLE users, logs"; got != want {
		t.Errorf("multi: got %q, want %q", got, want)
	}
}

func TestComments(t *testing.T) {
	if got, want := SetTableComment("users", "app users"),
		"COMMENT ON TABLE users IS 'app users'"; got != want {
		t.Errorf("table: got %q, want %q", got, want)
	}
	if got, want := SetColumnComment("users", "email", "primary email"),
		"COMMENT ON COLUMN users.email IS 'primary email'"; got != want {
		t.Errorf("column: got %q, want %q", got, want)
	}
	// Embedded single quotes are escaped.
	if got, want := SetTableComment("t", "it's fine"),
		"COMMENT ON TABLE t IS 'it''s fine'"; got != want {
		t.Errorf("escaped: got %q, want %q", got, want)
	}
}

func TestSequences(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "plain",
			got:  CreateSequence("order_num"),
			want: "CREATE SEQUENCE order_num",
		},
		{
			name: "start and increment",
			got:  CreateSequence("order_num", SequenceStart(1000), SequenceIncrement(1)),
			want: "CREATE SEQUENCE order_num INCREMENT BY 1 START WITH 1000",
		},
		{
			name: "increment only",
			got:  CreateSequence("s", SequenceIncrement(10)),
			want: "CREATE SEQUENCE s INCREMENT BY 10",
		},
		{
			name: "drop",
			got:  DropSequence("order_num"),
			want: "DROP SEQUENCE order_num",
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

package migrate

import "testing"

func TestChangeColumnDefault(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "string value",
			got:  ChangeColumnDefault("users", "status", "active"),
			want: "ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active'",
		},
		{
			name: "int value",
			got:  ChangeColumnDefault("users", "credits", 5),
			want: "ALTER TABLE users ALTER COLUMN credits SET DEFAULT 5",
		},
		{
			name: "bool value",
			got:  ChangeColumnDefault("users", "active", true),
			want: "ALTER TABLE users ALTER COLUMN active SET DEFAULT TRUE",
		},
		{
			name: "raw expression",
			got:  ChangeColumnDefaultRaw("users", "created_at", "CURRENT_TIMESTAMP"),
			want: "ALTER TABLE users ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP",
		},
		{
			name: "drop default",
			got:  DropColumnDefault("users", "status"),
			want: "ALTER TABLE users ALTER COLUMN status DROP DEFAULT",
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

func TestChangeColumnNull(t *testing.T) {
	if got, want := ChangeColumnNull("users", "email", false),
		"ALTER TABLE users ALTER COLUMN email SET NOT NULL"; got != want {
		t.Errorf("not null: got %q, want %q", got, want)
	}
	if got, want := ChangeColumnNull("users", "email", true),
		"ALTER TABLE users ALTER COLUMN email DROP NOT NULL"; got != want {
		t.Errorf("nullable: got %q, want %q", got, want)
	}
}

func TestRenameIndex(t *testing.T) {
	if got, want := RenameIndex("users", "old_idx", "new_idx"),
		"ALTER INDEX old_idx RENAME TO new_idx"; got != want {
		t.Errorf("ansi: got %q, want %q", got, want)
	}
	if got, want := NewSchema(MySQL).RenameIndex("users", "old_idx", "new_idx"),
		"ALTER TABLE `users` RENAME INDEX `old_idx` TO `new_idx`"; got != want {
		t.Errorf("mysql: got %q, want %q", got, want)
	}
	if got, want := NewSchema(Postgres).RenameIndex("users", "old_idx", "new_idx"),
		`ALTER INDEX "old_idx" RENAME TO "new_idx"`; got != want {
		t.Errorf("postgres: got %q, want %q", got, want)
	}
}

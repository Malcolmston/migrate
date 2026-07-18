package migrate

import "testing"

func TestExtensions(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "enable",
			got:  EnableExtension("pgcrypto"),
			want: `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		},
		{
			name: "enable hyphenated",
			got:  EnableExtension("uuid-ossp"),
			want: `CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
		},
		{
			name: "disable",
			got:  DisableExtension("pgcrypto"),
			want: `DROP EXTENSION IF EXISTS "pgcrypto"`,
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

func TestCreateEnumType(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "create ansi",
			got:  CreateEnum("mood", []string{"sad", "ok", "happy"}),
			want: "CREATE TYPE mood AS ENUM ('sad', 'ok', 'happy')",
		},
		{
			name: "create postgres",
			got:  NewSchema(Postgres).CreateEnum("mood", []string{"sad", "happy"}),
			want: `CREATE TYPE "mood" AS ENUM ('sad', 'happy')`,
		},
		{
			name: "drop",
			got:  DropEnum("mood"),
			want: "DROP TYPE mood",
		},
		{
			name: "add value append",
			got:  NewSchema(ANSI).AddEnumValue("mood", "excellent"),
			want: "ALTER TYPE mood ADD VALUE 'excellent'",
		},
		{
			name: "add value after",
			got:  NewSchema(ANSI).AddEnumValue("mood", "excellent", AfterValue("happy")),
			want: "ALTER TYPE mood ADD VALUE 'excellent' AFTER 'happy'",
		},
		{
			name: "add value before",
			got:  NewSchema(ANSI).AddEnumValue("mood", "terrible", BeforeValue("sad")),
			want: "ALTER TYPE mood ADD VALUE 'terrible' BEFORE 'sad'",
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

package migrate

import (
	"errors"
	"testing"
)

func TestDialectByName(t *testing.T) {
	cases := map[string]string{
		"ansi":       "ansi",
		"ANSI":       "ansi",
		"postgres":   "postgres",
		"postgresql": "postgres",
		"pg":         "postgres",
		"mysql":      "mysql",
		"mariadb":    "mysql",
		"sqlite":     "sqlite",
		"sqlite3":    "sqlite",
	}
	for name, want := range cases {
		d, err := DialectByName(name)
		if err != nil {
			t.Fatalf("DialectByName(%q): %v", name, err)
		}
		if d.Name() != want {
			t.Fatalf("DialectByName(%q).Name() = %q, want %q", name, d.Name(), want)
		}
	}
	if _, err := DialectByName("oracle"); !errors.Is(err, ErrUnknownDialect) {
		t.Fatalf("unknown dialect err = %v, want ErrUnknownDialect", err)
	}
}

func TestDialectQuoting(t *testing.T) {
	cases := []struct {
		d    Dialect
		in   string
		want string
	}{
		{ANSI, "email", "email"},
		{Postgres, "email", `"email"`},
		{MySQL, "email", "`email`"},
		{SQLite, "email", `"email"`},
		// Expressions are passed through untouched so functional indexes work.
		{Postgres, "lower(email)", "lower(email)"},
		{MySQL, "lower(email)", "lower(email)"},
	}
	for _, c := range cases {
		if got := c.d.Quote(c.in); got != c.want {
			t.Fatalf("%s.Quote(%q) = %q, want %q", c.d.Name(), c.in, got, c.want)
		}
	}
}

func TestDialectPlaceholders(t *testing.T) {
	if got := Postgres.Placeholder(1); got != "$1" {
		t.Fatalf("postgres placeholder = %q, want $1", got)
	}
	if got := Postgres.Placeholder(3); got != "$3" {
		t.Fatalf("postgres placeholder = %q, want $3", got)
	}
	for _, d := range []Dialect{ANSI, MySQL, SQLite} {
		if got := d.Placeholder(2); got != "?" {
			t.Fatalf("%s placeholder = %q, want ?", d.Name(), got)
		}
	}
}

func TestColumnTypeMapping(t *testing.T) {
	// abstract type -> per-dialect concrete spelling
	rows := []struct {
		build              func(*Table)
		ansi, pg, my, lite string
	}{
		{func(t *Table) { t.String("c", Limit(64)) }, "VARCHAR(64)", "VARCHAR(64)", "VARCHAR(64)", "VARCHAR(64)"},
		{func(t *Table) { t.Text("c") }, "TEXT", "TEXT", "TEXT", "TEXT"},
		{func(t *Table) { t.Integer("c") }, "INTEGER", "INTEGER", "INT", "INTEGER"},
		{func(t *Table) { t.BigInteger("c") }, "BIGINT", "BIGINT", "BIGINT", "BIGINT"},
		{func(t *Table) { t.Float("c") }, "DOUBLE PRECISION", "DOUBLE PRECISION", "DOUBLE", "REAL"},
		{func(t *Table) { t.Boolean("c") }, "BOOLEAN", "BOOLEAN", "TINYINT(1)", "BOOLEAN"},
		{func(t *Table) { t.Decimal("c", 10, 2) }, "DECIMAL(10,2)", "NUMERIC(10,2)", "DECIMAL(10,2)", "DECIMAL(10,2)"},
		{func(t *Table) { t.Binary("c") }, "BLOB", "BYTEA", "BLOB", "BLOB"},
		{func(t *Table) { t.JSON("c") }, "JSON", "JSON", "JSON", "JSON"},
		{func(t *Table) { t.JSONB("c") }, "JSONB", "JSONB", "JSON", "JSON"},
		{func(t *Table) { t.UUID("c") }, "UUID", "UUID", "CHAR(36)", "VARCHAR(36)"},
		{func(t *Table) { t.Timestamp("c", WithTimezone()) }, "TIMESTAMP WITH TIME ZONE", "TIMESTAMP WITH TIME ZONE", "DATETIME", "DATETIME"},
		{func(t *Table) { t.Timestamp("c", Precision(6)) }, "TIMESTAMP(6)", "TIMESTAMP(6)", "DATETIME(6)", "DATETIME"},
	}
	for i, r := range rows {
		got := renderType(ANSI, r.build)
		if got != r.ansi {
			t.Fatalf("row %d ANSI = %q, want %q", i, got, r.ansi)
		}
		if got := renderType(Postgres, r.build); got != r.pg {
			t.Fatalf("row %d Postgres = %q, want %q", i, got, r.pg)
		}
		if got := renderType(MySQL, r.build); got != r.my {
			t.Fatalf("row %d MySQL = %q, want %q", i, got, r.my)
		}
		if got := renderType(SQLite, r.build); got != r.lite {
			t.Fatalf("row %d SQLite = %q, want %q", i, got, r.lite)
		}
	}
}

// renderType builds a single-column table and returns the concrete SQL type of
// that column for the given dialect.
func renderType(d Dialect, build func(*Table)) string {
	tbl := &Table{name: "t", dialect: d}
	build(tbl)
	return d.columnType(tbl.columns[0].spec)
}

func TestArrayTypes(t *testing.T) {
	build := func(t *Table) { t.String("tags", Array()) }
	if got := renderType(Postgres, build); got != "VARCHAR(255)[]" {
		t.Fatalf("postgres array = %q", got)
	}
	if got := renderType(ANSI, build); got != "VARCHAR(255) ARRAY" {
		t.Fatalf("ansi array = %q", got)
	}
	if got := renderType(MySQL, build); got != "JSON" {
		t.Fatalf("mysql array = %q", got)
	}
	if got := renderType(SQLite, build); got != "JSON" {
		t.Fatalf("sqlite array = %q", got)
	}
}

func TestEnumTypes(t *testing.T) {
	build := func(t *Table) { t.Enum("status", []string{"active", "archived"}) }
	if got := renderType(MySQL, build); got != "ENUM('active', 'archived')" {
		t.Fatalf("mysql enum = %q", got)
	}
	if got := renderType(ANSI, build); got != "VARCHAR(255)" {
		t.Fatalf("ansi enum = %q", got)
	}
	// Postgres references a named type.
	pgBuild := func(t *Table) { t.EnumType("status", "status_type", []string{"active"}) }
	if got := renderType(Postgres, pgBuild); got != "status_type" {
		t.Fatalf("postgres enum = %q", got)
	}
}

func TestPostgresCreateTableQuotesAndSerial(t *testing.T) {
	s := NewSchema(Postgres)
	got := s.CreateTable("users", func(t *Table) {
		t.String("email", NotNull())
	})
	want := "CREATE TABLE \"users\" (\n" +
		"\t\"id\" BIGSERIAL PRIMARY KEY,\n" +
		"\t\"email\" VARCHAR(255) NOT NULL\n" +
		")"
	if got != want {
		t.Fatalf("postgres create table:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestMySQLCreateTableAutoIncrement(t *testing.T) {
	s := NewSchema(MySQL)
	got := s.CreateTable("users", func(t *Table) {
		t.String("email")
	})
	want := "CREATE TABLE `users` (\n" +
		"\t`id` BIGINT AUTO_INCREMENT PRIMARY KEY,\n" +
		"\t`email` VARCHAR(255)\n" +
		")"
	if got != want {
		t.Fatalf("mysql create table:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSQLiteCreateTable(t *testing.T) {
	s := NewSchema(SQLite)
	got := s.CreateTable("users", func(t *Table) {
		t.String("email")
	})
	want := "CREATE TABLE \"users\" (\n" +
		"\t\"id\" INTEGER PRIMARY KEY AUTOINCREMENT,\n" +
		"\t\"email\" VARCHAR(255)\n" +
		")"
	if got != want {
		t.Fatalf("sqlite create table:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestNewSchemaNilDefaultsToANSI(t *testing.T) {
	if NewSchema(nil).Dialect().Name() != "ansi" {
		t.Fatal("nil dialect should default to ANSI")
	}
}

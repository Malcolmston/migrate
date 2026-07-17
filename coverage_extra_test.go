package migrate

import (
	"strings"
	"testing"
)

func TestAlterTableAllHelpers(t *testing.T) {
	got := ChangeTable("t", func(a *AlterTable) {
		a.Text("bio")
		a.BigInteger("big")
		a.Boolean("flag")
		a.Timestamp("seen_at")
		a.Change("age", "BIGINT")
		a.RemoveIndex("index_t_on_old")
	})
	for _, want := range []string{
		"ALTER TABLE t ADD COLUMN bio TEXT",
		"ALTER TABLE t ADD COLUMN big BIGINT",
		"ALTER TABLE t ADD COLUMN flag BOOLEAN",
		"ALTER TABLE t ADD COLUMN seen_at TIMESTAMP",
		"ALTER TABLE t ALTER COLUMN age TYPE BIGINT",
		"DROP INDEX index_t_on_old",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func TestRemoveReference(t *testing.T) {
	if got, want := RemoveReference("comments", "author"), "ALTER TABLE comments DROP COLUMN author_id"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecimalAndTimeAndScaleTimezone(t *testing.T) {
	got := CreateTable("money", func(t *Table) {
		t.Decimal("amount", 12, 4, NotNull())
		t.Time("at", WithTimezone())
		t.Column("raw_scale", "NUMERIC", Precision(8), Scale(2))
	}, WithoutID())
	for _, want := range []string{
		"amount DECIMAL(12,4) NOT NULL",
		"at TIME WITH TIME ZONE",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func TestChangeRecorderDialect(t *testing.T) {
	r := &ChangeRecorder{schema: NewSchema(MySQL)}
	if r.Dialect().Name() != "mysql" {
		t.Fatalf("recorder dialect = %q", r.Dialect().Name())
	}
}

func TestSingularize(t *testing.T) {
	cases := map[string]string{"posts": "post", "s": "s", "user": "user", "boxes": "boxe"}
	for in, want := range cases {
		if got := singularize(in); got != want {
			t.Fatalf("singularize(%q) = %q, want %q", in, got, want)
		}
	}
}

package migrate

import "testing"

func TestAddCheckConstraint(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "generated name",
			got:  AddCheckConstraint("products", "price > 0"),
			want: "ALTER TABLE products ADD CONSTRAINT chk_products_price___0 CHECK (price > 0)",
		},
		{
			name: "explicit name",
			got:  AddCheckConstraint("products", "price > 0", ConstraintName("price_positive")),
			want: "ALTER TABLE products ADD CONSTRAINT price_positive CHECK (price > 0)",
		},
		{
			name: "remove reproduces name",
			got:  RemoveCheckConstraint("products", "price > 0"),
			want: "ALTER TABLE products DROP CONSTRAINT chk_products_price___0",
		},
		{
			name: "postgres quoting",
			got:  NewSchema(Postgres).AddCheckConstraint("products", "price > 0"),
			want: `ALTER TABLE "products" ADD CONSTRAINT "chk_products_price___0" CHECK (price > 0)`,
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

func TestUniqueConstraint(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "single column",
			got:  AddUniqueConstraint("users", []string{"email"}),
			want: "ALTER TABLE users ADD CONSTRAINT uniq_users_email UNIQUE (email)",
		},
		{
			name: "multi column",
			got:  AddUniqueConstraint("users", []string{"a", "b"}),
			want: "ALTER TABLE users ADD CONSTRAINT uniq_users_a_b UNIQUE (a, b)",
		},
		{
			name: "deferrable",
			got:  AddUniqueConstraint("users", []string{"email"}, Deferrable()),
			want: "ALTER TABLE users ADD CONSTRAINT uniq_users_email UNIQUE (email) DEFERRABLE INITIALLY DEFERRED",
		},
		{
			name: "explicit name",
			got:  AddUniqueConstraint("users", []string{"email"}, ConstraintName("users_email_key")),
			want: "ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email)",
		},
		{
			name: "remove reproduces name",
			got:  RemoveUniqueConstraint("users", []string{"a", "b"}),
			want: "ALTER TABLE users DROP CONSTRAINT uniq_users_a_b",
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

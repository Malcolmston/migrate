package migrate

import (
	"strings"
	"testing"
)

func TestAddForeignKeyWithActions(t *testing.T) {
	got := AddForeignKey("comments", "posts", OnDelete(Cascade), OnUpdate(Restrict))
	want := "ALTER TABLE comments ADD CONSTRAINT fk_comments_post_id " +
		"FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE ON UPDATE RESTRICT"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddForeignKeyDefaults(t *testing.T) {
	got := AddForeignKey("comments", "posts")
	want := "ALTER TABLE comments ADD CONSTRAINT fk_comments_post_id " +
		"FOREIGN KEY (post_id) REFERENCES posts (id)"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddForeignKeyOverrides(t *testing.T) {
	got := AddForeignKey("comments", "users",
		FKColumn("author_id"), FKPrimaryKey("uid"), FKName("comments_author_fk"), OnDelete(SetNull))
	want := "ALTER TABLE comments ADD CONSTRAINT comments_author_fk " +
		"FOREIGN KEY (author_id) REFERENCES users (uid) ON DELETE SET NULL"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveForeignKey(t *testing.T) {
	got := RemoveForeignKey("comments", "posts")
	want := "ALTER TABLE comments DROP CONSTRAINT fk_comments_post_id"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestAddReference(t *testing.T) {
	got := AddReference("comments", "author", ReferenceNotNull(), ReferenceIndex(), WithForeignKey(), ReferenceTable("users"))
	stmts := strings.Split(got, ";\n")
	want := []string{
		"ALTER TABLE comments ADD COLUMN author_id BIGINT NOT NULL",
		"CREATE INDEX index_comments_on_author_id ON comments (author_id)",
		"ALTER TABLE comments ADD CONSTRAINT fk_comments_author_id FOREIGN KEY (author_id) REFERENCES users (id)",
	}
	if len(stmts) != len(want) {
		t.Fatalf("got %d statements:\n%s", len(stmts), got)
	}
	for i := range want {
		if stmts[i] != want[i] {
			t.Fatalf("stmt %d:\ngot  %q\nwant %q", i, stmts[i], want[i])
		}
	}
}

func TestAddReferenceColumnOnly(t *testing.T) {
	got := AddReference("comments", "post")
	want := "ALTER TABLE comments ADD COLUMN post_id BIGINT"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestChangeColumnAnsiAndMySQL(t *testing.T) {
	if got, want := ChangeColumn("users", "age", "BIGINT"),
		"ALTER TABLE users ALTER COLUMN age TYPE BIGINT"; got != want {
		t.Fatalf("ansi change_column: got %q, want %q", got, want)
	}
	my := NewSchema(MySQL)
	if got, want := my.ChangeColumn("users", "age", "BIGINT", NotNull()),
		"ALTER TABLE `users` MODIFY COLUMN `age` BIGINT NOT NULL"; got != want {
		t.Fatalf("mysql change_column: got %q, want %q", got, want)
	}
}

func TestChangeTableBulk(t *testing.T) {
	got := ChangeTable("users", func(t *AlterTable) {
		t.String("nickname", Limit(50))
		t.Integer("login_count", NotNull(), Default(0))
		t.Rename("name", "full_name")
		t.Remove("legacy")
		t.Index([]string{"nickname"}, UniqueIndex())
	})
	want := strings.Join([]string{
		"ALTER TABLE users ADD COLUMN nickname VARCHAR(50)",
		"ALTER TABLE users ADD COLUMN login_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users RENAME COLUMN name TO full_name",
		"ALTER TABLE users DROP COLUMN legacy",
		"CREATE UNIQUE INDEX index_users_on_nickname ON users (nickname)",
	}, ";\n")
	if got != want {
		t.Fatalf("change_table:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestChangeTableReferencesAndTimestamps(t *testing.T) {
	got := ChangeTable("posts", func(t *AlterTable) {
		t.References("author", WithForeignKey(), ReferenceTable("users"))
		t.Timestamps()
	})
	for _, want := range []string{
		"ALTER TABLE posts ADD COLUMN author_id BIGINT",
		"ALTER TABLE posts ADD CONSTRAINT fk_posts_author_id FOREIGN KEY (author_id) REFERENCES users (id)",
		"ALTER TABLE posts ADD COLUMN created_at TIMESTAMP NOT NULL",
		"ALTER TABLE posts ADD COLUMN updated_at TIMESTAMP NOT NULL",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("change_table output missing %q:\n%s", want, got)
		}
	}
}

func TestAdvancedIndexes(t *testing.T) {
	// Partial index.
	if got, want := AddIndex("users", []string{"email"}, Where("deleted_at IS NULL")),
		"CREATE INDEX index_users_on_email ON users (email) WHERE deleted_at IS NULL"; got != want {
		t.Fatalf("partial index: got %q, want %q", got, want)
	}
	// USING method.
	pg := NewSchema(Postgres)
	if got, want := pg.AddIndex("docs", []string{"body"}, Using("gin")),
		`CREATE INDEX "index_docs_on_body" ON "docs" USING gin ("body")`; got != want {
		t.Fatalf("using index: got %q, want %q", got, want)
	}
	// Expression / functional index (expression passed through, name sanitized).
	got := AddIndex("users", []string{"lower(email)"})
	want := "CREATE INDEX index_users_on_lower_email_ ON users (lower(email))"
	if got != want {
		t.Fatalf("expression index: got %q, want %q", got, want)
	}
	// Unique + custom name + where combined.
	if got, want := AddIndex("users", []string{"email"}, UniqueIndex(), IndexName("uniq_email"), Where("active")),
		"CREATE UNIQUE INDEX uniq_email ON users (email) WHERE active"; got != want {
		t.Fatalf("combined index: got %q, want %q", got, want)
	}
}

func TestAddDropTimestamps(t *testing.T) {
	if got, want := AddTimestamps("users"), "ALTER TABLE users ADD COLUMN created_at TIMESTAMP NOT NULL;\n"+
		"ALTER TABLE users ADD COLUMN updated_at TIMESTAMP NOT NULL"; got != want {
		t.Fatalf("add timestamps: got %q, want %q", got, want)
	}
	if got, want := RemoveTimestamps("users"), "ALTER TABLE users DROP COLUMN updated_at;\n"+
		"ALTER TABLE users DROP COLUMN created_at"; got != want {
		t.Fatalf("remove timestamps: got %q, want %q", got, want)
	}
}

func TestDialectAwareDropAndRename(t *testing.T) {
	pg := NewSchema(Postgres)
	if got, want := pg.DropTable("users"), `DROP TABLE "users"`; got != want {
		t.Fatalf("pg drop: got %q, want %q", got, want)
	}
	if got, want := pg.RenameColumn("users", "a", "b"), `ALTER TABLE "users" RENAME COLUMN "a" TO "b"`; got != want {
		t.Fatalf("pg rename column: got %q, want %q", got, want)
	}
	if got, want := pg.RenameTable("users", "members"), `ALTER TABLE "users" RENAME TO "members"`; got != want {
		t.Fatalf("pg rename table: got %q, want %q", got, want)
	}
	if got, want := pg.DropTableIfExists("users"), `DROP TABLE IF EXISTS "users"`; got != want {
		t.Fatalf("pg drop if exists: got %q, want %q", got, want)
	}
}

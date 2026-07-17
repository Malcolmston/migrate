// Library content for the migrate documentation site. Mirrors the shape used by
// the malcolmston/go landing site's data.ts so the sibling sites stay in sync.
export interface Lib {
  id: string; name: string; icon: string; accent: string; pkg: string; node: string;
  repo: string; docs: string; tagline: string; blurb: string; tags: string[];
  features: string[]; node_code: string; go_code: string; integrate: string;
}

export const NODE_ACCENT = '#8cc84b';

export const MIGRATE: Lib = {
  id:"migrate", name:"Migrate", icon:'<i class="fa-solid fa-database"></i>', accent:"#8b5cf6",
  pkg:"github.com/malcolmston/migrate", node:"rails/rails",
  repo:"https://github.com/malcolmston/migrate", docs:"https://malcolmston.github.io/migrate/",
  tagline:"ActiveRecord-style schema migrations for Go.",
  blurb:"A small, dependency-free, ActiveRecord-flavoured schema-migration toolkit for Go built entirely on top of "+
    "the standard library's database/sql package — no third-party packages, no cgo. A Migration carries a uint64 "+
    "Version, a Name, and a pair of directions expressed either as a Go func(ctx, *sql.Tx) error or as raw SQL "+
    "text; a Migrator wraps a *sql.DB, maintains a schema_migrations bookkeeping table, and drives migrations "+
    "forward and backward with Migrate, Up, Down, Rollback, MigrateTo, Redo and Status. Every migration runs in "+
    "its own transaction, so a failure rolls back cleanly and re-running is idempotent. Migrations register "+
    "programmatically or load from a directory / io/fs.FS of <version>_<name>.up.sql / .down.sql pairs, and a "+
    "tiny schema DSL (CreateTable, AddColumn, AddIndex, typed column helpers, foreign keys) emits predictable, "+
    "greppable ANSI SQL.",
  tags:["database/sql","reversible","schema_migrations","transaction per migration","io/fs loading","schema DSL","zero deps","ANSI SQL"],
  features:[
    "Versioned, reversible migrations — a <code>Migration</code> with a <code>uint64</code> Version, applied ascending and rolled back descending",
    "Go <i>or</i> SQL directions — <code>Up</code>/<code>Down</code> as <code>func(ctx, *sql.Tx) error</code>, or <code>UpSQL</code>/<code>DownSQL</code> raw text",
    "Transaction per migration — a failure rolls back, does not record the version, and halts; re-running <code>Migrate</code> is idempotent",
    "Full command set — <code>Migrate</code>, <code>Up</code>, <code>Down</code>, <code>Rollback</code>, <code>MigrateTo</code>, <code>Redo</code> and <code>Status</code> on the <code>Migrator</code>",
    "File loading — <code>LoadDir</code> and <code>LoadFS</code> read <code>&lt;version&gt;_&lt;name&gt;.up.sql</code> / <code>.down.sql</code> pairs from any <code>io/fs.FS</code>",
    "Schema DSL — <code>CreateTable</code> with typed helpers (<code>String</code>, <code>Text</code>, <code>Integer</code>, <code>Boolean</code>, <code>Timestamps</code>) plus <code>AddColumn</code>, <code>AddIndex</code>, <code>DropTable</code>, <code>RenameColumn</code>",
    "Rails-style references &amp; foreign keys — <code>Table.References</code> with <code>WithForeignKey</code>, <code>ReferenceNotNull</code> and <code>ReferenceTable</code>",
    "Zero dependencies — pure Go standard library over <code>database/sql</code>, no cgo, nothing to audit but the toolchain"
  ],
  node_code:
`class CreateUsers < ActiveRecord::Migration[7.1]
  def change
    create_table :users do |t|
      t.string :email, null: false
      t.timestamps
    end
  end
end

# bin/rails db:migrate`,
  go_code:
`import "github.com/malcolmston/migrate"

mg := migrate.New(db) // any database/sql *sql.DB
mg.Register(migrate.Migration{
    Version: 20240101,
    Name:    "create_users",
    UpSQL: migrate.CreateTable("users", func(t *migrate.Table) {
        t.String("email", migrate.NotNull(), migrate.Unique())
        t.Timestamps()
    }),
    DownSQL: migrate.DropTable("users"),
})
if err := mg.Migrate(context.Background()); err != nil {
    log.Fatal(err)
}`,
  integrate:
`<span class="tok-c">// Load .up.sql/.down.sql pairs from a directory and register them.</span>
migs, err := migrate.LoadDir("migrations")
if err != nil {
    log.Fatal(err)
}
mg := migrate.New(db, migrate.WithTable("schema_migrations"))
mg.Register(migs...)

<span class="tok-c">// Apply everything pending, ascending — each in its own transaction.</span>
if err := mg.Migrate(ctx); err != nil {
    log.Fatal(err)
}

<span class="tok-c">// Report which versions are applied vs pending.</span>
statuses, _ := mg.Status(ctx)
for _, s := range statuses {
    log.Printf("%d %s applied=%v", s.Version, s.Name, s.Applied)
}

<span class="tok-c">// Undo the most recent migration, then move to an exact version.</span>
if err := mg.Rollback(ctx, 1); err != nil {
    log.Fatal(err)
}
if err := mg.MigrateTo(ctx, 20240101); err != nil {
    log.Fatal(err)
}`
};

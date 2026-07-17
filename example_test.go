package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

// Example demonstrates a full migrate / status / rollback cycle against the
// in-memory test driver. Real programs would open a driver such as "sqlite" or
// "mysql" instead of "memdb".
func Example() {
	ctx := context.Background()
	resetMemDB("example")
	db, err := sql.Open("memdb", "example")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mg := New(db)
	err = mg.Register(
		Migration{
			Version: 20240101,
			Name:    "create_users",
			UpSQL: CreateTable("users", func(t *Table) {
				t.String("email", NotNull(), Unique())
				t.Timestamps()
			}),
			DownSQL: DropTable("users"),
		},
		Migration{
			Version: 20240102,
			Name:    "add_users_name",
			UpSQL:   AddColumn("users", "name", "VARCHAR", Limit(100)),
			DownSQL: DropColumn("users", "name"),
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := mg.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	printStatus(ctx, mg, "after migrate")

	if err := mg.Rollback(ctx, 1); err != nil {
		log.Fatal(err)
	}
	printStatus(ctx, mg, "after rollback 1")

	// Output:
	// after migrate:
	//   20240101 create_users applied
	//   20240102 add_users_name applied
	// after rollback 1:
	//   20240101 create_users applied
	//   20240102 add_users_name pending
}

func printStatus(ctx context.Context, mg *Migrator, label string) {
	statuses, err := mg.Status(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s:\n", label)
	for _, s := range statuses {
		state := "pending"
		if s.Applied {
			state = "applied"
		}
		fmt.Printf("  %d %s %s\n", s.Version, s.Name, state)
	}
}

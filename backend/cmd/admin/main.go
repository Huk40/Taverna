// This command runs on the trusted server. Registration cannot assign roles.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	email := flag.String("email", "", "Existing account email")
	claim := flag.Bool("claim-legacy", false, "Assign only projects without an owner to this account")
	flag.Parse()
	if err := run(strings.ToLower(strings.TrimSpace(*email)), *claim); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(email string, claim bool) error {
	if email == "" {
		return fmt.Errorf("use: admin -email owner@example.com [-claim-legacy]")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `UPDATE users SET role='admin',updated_at=NOW() WHERE LOWER(email)=$1 RETURNING id::text`, email).Scan(&id)
	if err != nil {
		return fmt.Errorf("register this account first: %w", err)
	}
	if claim {
		if _, err = tx.Exec(ctx, `UPDATE projects SET owner_id=$1 WHERE owner_id IS NULL`, id); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Println("Administrator role assigned. Existing sessions will see the role on their next request.")
	return nil
}

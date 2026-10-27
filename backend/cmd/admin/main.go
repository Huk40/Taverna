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
	flag.Parse()
	if err := run(strings.ToLower(strings.TrimSpace(*email))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(email string) error {
	if email == "" {
		return fmt.Errorf("use: admin -email owner@example.com")
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
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Println("Administrator role assigned. Existing sessions will see the role on their next request.")
	return nil
}

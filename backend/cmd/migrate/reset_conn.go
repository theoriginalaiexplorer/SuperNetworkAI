//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	pool, _ := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	defer pool.Close()
	tag, err := pool.Exec(context.Background(),
		`UPDATE connections SET status='pending'
		 WHERE requester_id='550e8400-e29b-41d4-a716-446655440000'
		   AND recipient_id='660e8500-e29b-41d4-a716-446655440001'`)
	fmt.Printf("rows affected: %d, err: %v\n", tag.RowsAffected(), err)
}

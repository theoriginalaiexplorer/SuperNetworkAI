package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	dir := filepath.Join("db", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		log.Fatalf("glob: %v", err)
	}
	sort.Strings(files)
	fmt.Printf("found %d migration files\n", len(files))

	// Enable pgvector first (Neon supports it; Supabase migration 001 was a manual step)
	fmt.Print("enabling vector extension ... ")
	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector;")
	if err != nil {
		fmt.Printf("WARN: %v\n", err)
	} else {
		fmt.Println("OK")
	}

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read %s: %v", f, err)
		}
		base := filepath.Base(f)
		fmt.Printf("applying %s ... ", base)
		_, err = pool.Exec(ctx, string(sql))
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				fmt.Println("skipped (exists)")
				continue
			}
			fmt.Printf("FAILED: %v\n", err)
			continue
		}
		fmt.Println("OK")
	}
	fmt.Println("migrations done")
}

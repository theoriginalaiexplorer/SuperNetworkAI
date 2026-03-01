package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all required environment variables. The process exits
// immediately if any required variable is missing.
type Config struct {
	// Server
	Port string

	// Database
	DatabaseURL string

	// Supabase
	SupabaseURL string
	SupabaseKey string // service-role key

	// AI / Embeddings
	GroqAPIKey          string
	OllamaBaseURL       string
	NomicAPIKey         string
	EmbeddingProvider   string // "ollama" | "nomic"

	// Auth
	WSTokenSecret      string // HMAC key for WebSocket tokens

	// Internal
	InternalAPISecret  string // X-Internal-Secret header for Cloud Scheduler

	// File uploads
	UploadthingSecret string
}

// Load reads .env (if present) and then env vars. Exits if any required
// variable is absent.
func Load() *Config {
	// .env is optional — in prod, vars are injected directly
	_ = godotenv.Load()

	cfg := &Config{
		Port:              getEnvOrDefault("PORT", "3001"),
		DatabaseURL:       requireEnv("DATABASE_URL"),
		SupabaseURL:       requireEnv("SUPABASE_URL"),
		SupabaseKey:       requireEnv("SUPABASE_KEY"),
		GroqAPIKey:        requireEnv("GROQ_API_KEY"),
		OllamaBaseURL:     getEnvOrDefault("OLLAMA_BASE_URL", "http://localhost:11434"),
		NomicAPIKey:       getEnvOrDefault("NOMIC_API_KEY", ""),
		EmbeddingProvider: getEnvOrDefault("EMBEDDING_PROVIDER", "ollama"),
		WSTokenSecret:     requireEnv("WS_TOKEN_SECRET"),
		InternalAPISecret: requireEnv("INTERNAL_API_SECRET"),
		UploadthingSecret: getEnvOrDefault("UPLOADTHING_SECRET", ""),
	}

	return cfg
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "FATAL: required env var %q is not set\n", key)
		os.Exit(1)
	}
	return v
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

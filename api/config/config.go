package config

import (
	"os"
)

// Config contiene toda la configuración de la aplicación leída desde variables de entorno.
// Se carga una vez al inicio y se pasa a todos los componentes que la necesiten.
type Config struct {
	// Servidor
	Port string

	// Base de datos PostgreSQL
	PostgresURL string

	// Redis
	RedisURL string

	// MinIO
	MinioEndpoint   string
	MinioAccessKey  string
	MinioSecretKey  string
	MinioBucketOrig string
	MinioBucketBund string
	MinioUseSSL     bool

	// JWT
	JWTSecret string
}

// Load lee las variables de entorno y devuelve la configuración.
// Usa valores por defecto seguros para desarrollo local.
func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		PostgresURL:     getEnv("POSTGRES_URL", "postgres://okf:okfpassword@localhost:5432/okf?sslmode=disable"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		MinioEndpoint:   getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:  getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:  getEnv("MINIO_SECRET_KEY", "minioadmin123"),
		MinioBucketOrig: getEnv("MINIO_BUCKET_ORIGINALS", "originals"),
		MinioBucketBund: getEnv("MINIO_BUCKET_BUNDLES", "bundles"),
		MinioUseSSL:     getEnv("MINIO_USE_SSL", "false") == "true",
		JWTSecret:       getEnv("JWT_SECRET", "default-insecure-secret"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

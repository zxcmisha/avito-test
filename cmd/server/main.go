package main

import (
	"log"
	"net/http"
	"os"

	_ "avito/docs"
	"avito/internal/app"
)

func main() {
	dsn := getenv("DATABASE_URL", "postgres://postgres:postgres@db:5432/booking?sslmode=disable")
	migrationsPath := getenv("MIGRATIONS_PATH", "file:///app/internal/migrations")
	redisAddr := getenv("REDIS_ADDR", "redis:6379")
	redisPassword := getenv("REDIS_PASSWORD", "")
	redisDB := 0
	jwtSecret := getenv("JWT_SECRET", "dev-secret")
	port := getenv("PORT", "8080")

	if err := app.RunMigrations(migrationsPath, dsn); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store, err := app.NewPostgresStore(dsn)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer store.Close()

	cache, err := app.NewRedisSlotsCache(redisAddr, redisPassword, redisDB)
	if err != nil {
		log.Printf("redis unavailable, fallback to noop cache: %v", err)
	}
	var slotsCache app.SlotsCache = app.NoopSlotsCache{}
	if cache != nil {
		defer cache.Close()
		slotsCache = cache
	}

	srv := app.NewServer(store, app.NewJWTManager(jwtSecret), slotsCache)
	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv.Router()); err != nil {
		log.Fatal(err)
	}
}

func getenv(k, fallback string) string {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	return v
}

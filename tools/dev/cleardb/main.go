// Cleardb — очистка всех данных БД (PostgreSQL + Redis). Только для разработки.
// Защита: задайте CONFIRM_CLEAR_DB=development (или yes) и не задавайте APP_ENV=production.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/messenger/internal/config"
	redisstorage "github.com/messenger/internal/storage/redis"
)

const confirmEnv = "CONFIRM_CLEAR_DB"
const appEnvKey = "APP_ENV"

func main() {
	confirm := os.Getenv(confirmEnv)
	appEnv := os.Getenv(appEnvKey)

	if appEnv == "production" {
		log.Fatalf("[cleardb] запрещено в production (APP_ENV=production). Удалите APP_ENV или не запускайте cleardb.")
	}
	if confirm != "development" && confirm != "yes" {
		log.Fatalf("[cleardb] опасная операция. Для подтверждения задайте %s=development или %s=yes (только для локальной разработки).", confirmEnv, confirmEnv)
	}

	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("подключение к БД: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping БД: %v (запустите PostgreSQL)", err)
	}

	sql := `TRUNCATE TABLE
		sessions,
		message_reactions,
		pinned_messages,
		user_favorite_chats,
		chat_members,
		messages,
		chats,
		users
		RESTART IDENTITY CASCADE`
	if _, err := pool.Exec(ctx, sql); err != nil {
		log.Fatalf("очистка БД: %v", err)
	}
	log.Println("PostgreSQL: данные очищены.")

	if cfg.Redis.URL != "" {
		redisCtx, redisCancel := context.WithTimeout(context.Background(), 5*time.Second)
		redisClient, err := redisstorage.New(redisCtx, cfg.Redis.URL)
		redisCancel()
		if err != nil {
			log.Printf("Redis недоступен (%v), пропуск очистки Redis.", err)
		} else {
			defer redisClient.Close()
			if err := redisClient.FlushDB(ctx); err != nil {
				log.Fatalf("очистка Redis: %v", err)
			}
			log.Println("Redis: данные очищены.")
		}
	} else {
		log.Println("REDIS_URL не задан, пропуск Redis.")
	}

	log.Println("Все базы данных очищены. Только для разработки.")
	os.Exit(0)
}

package startup

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/messenger/internal/logger"
)

// ConnectDBWithRetry подключается к Postgres с повторами; при недоступности БД не роняет процесс сразу.
// logPrefix добавляется к сообщениям лога (например "auth: ").
func ConnectDBWithRetry(poolCfg *pgxpool.Config, maxWait time.Duration, logPrefix string) *pgxpool.Pool {
	deadline := time.Now().Add(maxWait)
	backoff := 2 * time.Second
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
		cancel()
		if err != nil {
			if time.Now().After(deadline) {
				logger.Errorf("%sconnect to db (gave up after %v): %v", logPrefix, maxWait, err)
				os.Exit(1)
			}
			logger.Errorf("%sdb connect failed, retry in %v: %v", logPrefix, backoff, err)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = pool.Ping(pingCtx)
		pingCancel()
		if err != nil {
			pool.Close()
			if time.Now().After(deadline) {
				logger.Errorf("%sdb ping (gave up after %v): %v", logPrefix, maxWait, err)
				os.Exit(1)
			}
			logger.Errorf("%sdb ping failed, retry in %v: %v", logPrefix, backoff, err)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		return pool
	}
}

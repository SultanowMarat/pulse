package startup

import (
	"context"
	"os"
	"time"

	redisstorage "github.com/messenger/internal/storage/redis"
	"github.com/messenger/internal/logger"
)

// ConnectRedisWithRetry подключается к Redis с повторами.
// logPrefix добавляется к сообщениям лога (например "auth: ").
func ConnectRedisWithRetry(redisURL string, maxWait time.Duration, logPrefix string) *redisstorage.Client {
	deadline := time.Now().Add(maxWait)
	backoff := 2 * time.Second
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, err := redisstorage.New(ctx, redisURL)
		cancel()
		if err != nil {
			if time.Now().After(deadline) {
				logger.Errorf("%sredis (gave up after %v): %v", logPrefix, maxWait, err)
				os.Exit(1)
			}
			logger.Errorf("%sredis connect failed, retry in %v: %v", logPrefix, backoff, err)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		return client
	}
}

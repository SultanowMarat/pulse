package startup

import (
	"context"
	"os"
	"time"

	redisstorage "github.com/pulse/internal/storage/redis"
	"github.com/pulse/internal/logger"
)

// ConnectRedisWithRetry ?>4:;ÑŽÑ‡05Ñ‚AO : Redis A ?>2Ñ‚>Ñ€0<8.
// logPrefix 4>102;O5Ñ‚AO : A>>1Ñ‰5=8O< ;>30 (=0?Ñ€8<5Ñ€ "auth: ").
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

package startup

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulse/internal/logger"
)

// ConnectDBWithRetry ?>4:;ÑŽÑ‡05Ñ‚AO : Postgres A ?>2Ñ‚>Ñ€0<8; ?Ñ€8 =54>AÑ‚Ñƒ?=>AÑ‚8 Ð‘Ð” =5 Ñ€>=O5Ñ‚ ?Ñ€>Ñ†5AA AÑ€07Ñƒ.
// logPrefix 4>102;O5Ñ‚AO : A>>1Ñ‰5=8O< ;>30 (=0?Ñ€8<5Ñ€ "auth: ").
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

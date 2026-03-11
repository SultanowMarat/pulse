package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pulse/internal/logger"
	"github.com/pulse/internal/push"
	"gopkg.in/yaml.v3"
)

// loadEnv Ñ‡8Ñ‚05Ñ‚ .env Ñ‚>;ÑŒ:> 2=5 production (2 :>=Ñ‚59=5Ñ€5/prod :>=Ñ„83 Ñ‚>;ÑŒ:> 87 env).
func loadEnv() {
	if os.Getenv("APP_ENV") == "production" {
		return
	}
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 5; i++ {
		path := dir + "/.env"
		f, err := os.Open(path)
		if err == nil {
			loadEnvFrom(f)
			f.Close()
			return
		}
		parent := strings.TrimSuffix(dir, "/")
		if idx := strings.LastIndex(parent, "/"); idx <= 0 {
			return
		} else {
			dir = parent[:idx]
			if dir == "" {
				dir = "/"
			}
		}
	}
}

func loadEnvFrom(f *os.File) {
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

// CacheConfig â€” =0AÑ‚Ñ€>9:8 :5Ñˆ0 (A?8A:8 Ñ‡0Ñ‚>2, 871Ñ€0==>5 =0 :;85=Ñ‚5).
type CacheConfig struct {
	TTLMinutes int `yaml:"ttl_minutes"`
}

// RedisConfig â€” Redis (OTP, rate limit, A5:Ñ€5Ñ‚Ñ‹ A5AA89).
type RedisConfig struct {
	URL string `yaml:"url"`
}

// SMTPConfig â€” SMTP 4;O >Ñ‚?Ñ€02:8 OTP (/=45:A.ÐŸ>Ñ‡Ñ‚0 8 4Ñ€.).
type SMTPConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	FromEmail string `yaml:"from_email"`
	FromName  string `yaml:"from_name"`
	UseTLS    bool   `yaml:"use_tls"`
}

// DatabaseConfig â€” =0AÑ‚Ñ€>9:8 ?>4:;ÑŽÑ‡5=8O : Ð‘Ð”.
type DatabaseConfig struct {
	URL            string `yaml:"database_url"`
	MaxConnections int    `yaml:"db_max_connections"`
}

// Config A>45Ñ€68Ñ‚ =0AÑ‚Ñ€>9:8 ?Ñ€8;>65=8O, Ð‘Ð” 8 :5Ñˆ0.
// ÐŸÑ€8>Ñ€8Ñ‚5Ñ‚: ?5Ñ€5<5==Ñ‹5 >:Ñ€Ñƒ65=8O > YAML-Ñ„09;Ñ‹ > 7=0Ñ‡5=8O ?> Ñƒ<>;Ñ‡0=8ÑŽ.
type Config struct {
	// !5Ñ€25Ñ€
	ServerAddr   string        `yaml:"server_addr"`
	ReadTimeout  time.Duration `yaml:"-"`
	WriteTimeout time.Duration `yaml:"-"`
	IdleTimeout  time.Duration `yaml:"-"`

	// Ð‘070 40==Ñ‹Ñ… (703Ñ€Ñƒ605Ñ‚AO 87 config/database.yaml)
	Database DatabaseConfig `yaml:"-"`

	// $09;Ñ‹
	UploadDir     string `yaml:"upload_dir"`
	MaxUploadSize int64  `yaml:"-"`

	// WebSocket
	MaxWSConnections int `yaml:"max_ws_connections"`
	WSSendBufferSize int `yaml:"ws_send_buffer_size"`
	WSWriteTimeout   int `yaml:"ws_write_timeout"`
	WSPongTimeout    int `yaml:"ws_pong_timeout"`
	WSMaxMessageSize int `yaml:"ws_max_message_size"`

	// CORS
	CORSAllowedOrigins string `yaml:"cors_allowed_origins"`

	// Ð›>38Ñ€>20=85
	LogLevel string `yaml:"log_level"`

	// Ðš5Ñˆ (703Ñ€Ñƒ605Ñ‚AO 87 config/cache.yaml)
	Cache CacheConfig `yaml:"-"`

	// Redis 8 SMTP (4;O <8:Ñ€>A5Ñ€28A0 auth 8 >?Ñ†8>=0;ÑŒ=> 4;O API)
	Redis RedisConfig `yaml:"-"`
	SMTP  SMTPConfig  `yaml:"-"`

	// AuthServiceURL â€” URL <8:Ñ€>A5Ñ€28A0 02Ñ‚>Ñ€870Ñ†88 (4;O API: ?Ñ€>25Ñ€:0 A5AA89).
	AuthServiceURL string `yaml:"-"`

	// PushServiceURL â€” URL <8:Ñ€>A5Ñ€28A0 ?ÑƒÑˆ-Ñƒ254><;5=89. ÐŸÑƒAÑ‚>9 â€” ?ÑƒÑˆ8 >Ñ‚:;ÑŽÑ‡5=Ñ‹.
	PushServiceURL string `yaml:"-"`
	// PushVAPIDPublicKey â€” ?Ñƒ1;8Ñ‡=Ñ‹9 VAPID-:;ÑŽÑ‡ 4;O ?>4?8A:8 2 1Ñ€0Ñƒ75Ñ€5 (>Ñ‚40Ñ‘Ñ‚AO Ñ„Ñ€>=Ñ‚Ñƒ).
	PushVAPIDPublicKey string `yaml:"-"`

	// FileServiceURL â€” URL <8:Ñ€>A5Ñ€28A0 Ñ„09;>2 (upload/serve). ÐŸÑƒAÑ‚>9 â€” Ñ„09;Ñ‹ >1Ñ€010Ñ‚Ñ‹20ÑŽÑ‚AO 2 API.
	FileServiceURL string `yaml:"-"`
	// AudioServiceURL â€” URL <8:Ñ€>A5Ñ€28A0 3>;>A>2Ñ‹Ñ… A>>1Ñ‰5=89 (upload/serve).
	AudioServiceURL string `yaml:"-"`
	// App status flags for maintenance/degradation banner in clients.
	AppMaintenance   bool   `yaml:"-"`
	AppReadOnly      bool   `yaml:"-"`
	AppDegradation   bool   `yaml:"-"`
	AppStatusMessage string `yaml:"-"`
}

// DatabaseURL 2>72Ñ€0Ñ‰05Ñ‚ AÑ‚Ñ€>:Ñƒ ?>4:;ÑŽÑ‡5=8O : Ð‘Ð” (Ñƒ4>1=> 4;O :>40, >6840ÑŽÑ‰53> cfg.DatabaseURL).
func (c *Config) DatabaseURL() string { return c.Database.URL }

// DBMaxConnections 2>72Ñ€0Ñ‰05Ñ‚ <0:A8<0;ÑŒ=>5 Ñ‡8A;> A>548=5=89 2 ?Ñƒ;5.
func (c *Config) DBMaxConnections() int {
	if c.Database.MaxConnections <= 0 {
		return 20
	}
	return c.Database.MaxConnections
}

// yamlConfig â€” ?Ñ€><56ÑƒÑ‚>Ñ‡=0O AÑ‚Ñ€Ñƒ:Ñ‚ÑƒÑ€0 4;O ?0Ñ€A8=30 app YAML (157 Ð‘Ð”).
type yamlConfig struct {
	ServerAddr         string `yaml:"server_addr"`
	ReadTimeout        int    `yaml:"read_timeout"`
	WriteTimeout       int    `yaml:"write_timeout"`
	IdleTimeout        int    `yaml:"idle_timeout"`
	UploadDir          string `yaml:"upload_dir"`
	MaxUploadSizeMB    int    `yaml:"max_upload_size_mb"`
	MaxWSConnections   int    `yaml:"max_ws_connections"`
	WSSendBufferSize   int    `yaml:"ws_send_buffer_size"`
	WSWriteTimeout     int    `yaml:"ws_write_timeout"`
	WSPongTimeout      int    `yaml:"ws_pong_timeout"`
	WSMaxMessageSize   int    `yaml:"ws_max_message_size"`
	CORSAllowedOrigins string `yaml:"cors_allowed_origins"`
	LogLevel           string `yaml:"log_level"`
}

// Load 703Ñ€Ñƒ605Ñ‚ :>=Ñ„83ÑƒÑ€0Ñ†8ÑŽ.
// !=0Ñ‡0;0 ?>43Ñ€Ñƒ60ÑŽÑ‚AO ?5Ñ€5<5==Ñ‹5 87 .env (5A;8 5AÑ‚ÑŒ), 70Ñ‚5< YAML 8 env (env 8<55Ñ‚ ?Ñ€8>Ñ€8Ñ‚5Ñ‚).
func Load() *Config {
	loadEnv()
	// Ð—=0Ñ‡5=8O ?> Ñƒ<>;Ñ‡0=8ÑŽ
	yc := yamlConfig{
		ServerAddr:         ":8080",
		ReadTimeout:        15,
		WriteTimeout:       15,
		IdleTimeout:        60,
		UploadDir:          "./uploads",
		MaxUploadSizeMB:    20,
		MaxWSConnections:   10000,
		WSSendBufferSize:   256,
		WSWriteTimeout:     10,
		WSPongTimeout:      60,
		WSMaxMessageSize:   4096,
		CORSAllowedOrigins: "*",
		LogLevel:           "info",
	}

	// Ð—03Ñ€Ñƒ7:0 :>=Ñ„83ÑƒÑ€0Ñ†88 ?Ñ€8;>65=8O: CONFIG_PATH â†’ config/api.yaml / config/auth.yaml
	appPaths := []string{os.Getenv("CONFIG_PATH"), "config/api.yaml", "config/auth.yaml"}
	for _, path := range appPaths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &yc); err != nil {
			logger.Errorf("config: >Ñˆ81:0 ?0Ñ€A8=30 %s: %v (8A?>;ÑŒ7ÑƒÑŽÑ‚AO 7=0Ñ‡5=8O ?> Ñƒ<>;Ñ‡0=8ÑŽ)", path, err)
		} else {
			logger.Infof("config: 703Ñ€Ñƒ65= %s", path)
		}
		break
	}

	// Ð—03Ñ€Ñƒ7:0 :>=Ñ„83ÑƒÑ€0Ñ†88 Ð‘Ð”: DATABASE_CONFIG_PATH > config/database.yaml > config/database.yaml.example
	dbURL := "postgres://pulse:pulse_secret@localhost:5432/pulse?sslmode=disable"
	dbMaxConn := 20
	dbPaths := []string{os.Getenv("DATABASE_CONFIG_PATH"), "config/database.yaml", "config/database.yaml.example"}
	for _, path := range dbPaths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var dc struct {
			URL            string `yaml:"database_url"`
			MaxConnections int    `yaml:"db_max_connections"`
		}
		if err := yaml.Unmarshal(data, &dc); err != nil {
			logger.Errorf("config: >Ñˆ81:0 ?0Ñ€A8=30 %s: %v (Ð‘Ð”: 7=0Ñ‡5=8O ?> Ñƒ<>;Ñ‡0=8ÑŽ)", path, err)
		} else {
			if dc.URL != "" {
				dbURL = dc.URL
			}
			if dc.MaxConnections > 0 {
				dbMaxConn = dc.MaxConnections
			}
			logger.Infof("config: 703Ñ€Ñƒ65= %s", path)
		}
		break
	}
	dbURL = envStr("DATABASE_URL", dbURL)
	dbMaxConn = envInt("DB_MAX_CONNECTIONS", dbMaxConn)
	if dbMaxConn <= 0 {
		dbMaxConn = 20
	}

	// Ð—03Ñ€Ñƒ7:0 :>=Ñ„83ÑƒÑ€0Ñ†88 :5Ñˆ0: CACHE_CONFIG_PATH > config/cache.yaml
	cacheDefault := 10
	cachePaths := []string{os.Getenv("CACHE_CONFIG_PATH"), "config/cache.yaml"}
	for _, path := range cachePaths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cc struct {
			TTLMinutes int `yaml:"ttl_minutes"`
		}
		if err := yaml.Unmarshal(data, &cc); err != nil {
			logger.Errorf("config: >Ñˆ81:0 ?0Ñ€A8=30 %s: %v (:5Ñˆ: 7=0Ñ‡5=85 ?> Ñƒ<>;Ñ‡0=8ÑŽ)", path, err)
		} else {
			cacheDefault = cc.TTLMinutes
			if cacheDefault <= 0 {
				cacheDefault = 10
			}
			logger.Infof("config: 703Ñ€Ñƒ65= %s", path)
		}
		break
	}
	cacheTTL := envInt("CACHE_TTL_MINUTES", cacheDefault)
	if cacheTTL <= 0 {
		cacheTTL = 10
	}

	redisURL := envStr("REDIS_URL", "redis://localhost:6379")
	smtpCfg := SMTPConfig{
		Host:      envStr("SMTP_HOST", "smtp.yandex.ru"),
		Port:      envInt("SMTP_PORT", 587),
		Username:  envStr("SMTP_USERNAME", ""),
		Password:  envStr("SMTP_PASSWORD", ""),
		FromEmail: envStr("SMTP_FROM_EMAIL", ""),
		FromName:  envStr("SMTP_FROM_NAME", "Auth Service"),
		UseTLS:    true,
	}
	authServiceURL := envStr("AUTH_SERVICE_URL", "http://localhost:8081")
	pushServiceURL := envStr("PUSH_SERVICE_URL", "")
	pushVAPIDPublic := envStr("PUSH_VAPID_PUBLIC_KEY", "")
	if pushVAPIDPublic == "" {
		if keys, err := push.EnsureVAPIDKeys(""); err == nil {
			pushVAPIDPublic = keys.PublicKey
		}
	}

	cfg := &Config{
		ServerAddr:         envStr("SERVER_ADDR", yc.ServerAddr),
		ReadTimeout:        time.Duration(envInt("READ_TIMEOUT", yc.ReadTimeout)) * time.Second,
		WriteTimeout:       time.Duration(envInt("WRITE_TIMEOUT", yc.WriteTimeout)) * time.Second,
		IdleTimeout:        time.Duration(envInt("IDLE_TIMEOUT", yc.IdleTimeout)) * time.Second,
		Database:           DatabaseConfig{URL: dbURL, MaxConnections: dbMaxConn},
		UploadDir:          envStr("UPLOAD_DIR", yc.UploadDir),
		MaxUploadSize:      int64(envInt("MAX_UPLOAD_SIZE_MB", yc.MaxUploadSizeMB)) << 20,
		MaxWSConnections:   envInt("MAX_WS_CONNECTIONS", yc.MaxWSConnections),
		WSSendBufferSize:   envInt("WS_SEND_BUFFER_SIZE", yc.WSSendBufferSize),
		WSWriteTimeout:     envInt("WS_WRITE_TIMEOUT", yc.WSWriteTimeout),
		WSPongTimeout:      envInt("WS_PONG_TIMEOUT", yc.WSPongTimeout),
		WSMaxMessageSize:   envInt("WS_MAX_MESSAGE_SIZE", yc.WSMaxMessageSize),
		CORSAllowedOrigins: envStr("CORS_ALLOWED_ORIGINS", yc.CORSAllowedOrigins),
		LogLevel:           envStr("LOG_LEVEL", yc.LogLevel),
		Cache:              CacheConfig{TTLMinutes: cacheTTL},
		Redis:              RedisConfig{URL: redisURL},
		SMTP:               smtpCfg,
		AuthServiceURL:     authServiceURL,
		PushServiceURL:     pushServiceURL,
		PushVAPIDPublicKey: pushVAPIDPublic,
		FileServiceURL:     envStr("FILE_SERVICE_URL", ""),
		AudioServiceURL:    envStr("AUDIO_SERVICE_URL", ""),
		AppMaintenance:     envBool("APP_MAINTENANCE", false),
		AppReadOnly:        envBool("APP_READ_ONLY", false),
		AppDegradation:     envBool("APP_DEGRADATION", false),
		AppStatusMessage:   strings.TrimSpace(envStr("APP_STATUS_MESSAGE", "")),
	}

	if os.Getenv("APP_ENV") == "production" {
		if cfg.CORSAllowedOrigins == "" || cfg.CORSAllowedOrigins == "*" {
			logger.Errorf("config: 2 production 70409Ñ‚5 CORS_ALLOWED_ORIGINS (O2=Ñ‹9 A?8A>: origins, =5 *)")
			// 5 Ñ€>=O5< ?Ñ€>Ñ†5AA â€” A09Ñ‚ 4>;65= >Ñ‚:Ñ€Ñ‹20Ñ‚ÑŒAO; CORS <>6=> 7040Ñ‚ÑŒ ?>765
		}
		if strings.Contains(cfg.Database.URL, "pulse_secret") && strings.Contains(cfg.Database.URL, "localhost") {
			logger.Errorf("config: 2 production 70409Ñ‚5 DATABASE_URL (=5 8A?>;ÑŒ7Ñƒ9Ñ‚5 45Ñ„>;Ñ‚ 4;O Ñ€07Ñ€01>Ñ‚:8)")
			os.Exit(1)
		}
	}

	return cfg
}

// envStr 2>72Ñ€0Ñ‰05Ñ‚ 7=0Ñ‡5=85 ?5Ñ€5<5==>9 >:Ñ€Ñƒ65=8O 8;8 fallback.
func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt 2>72Ñ€0Ñ‰05Ñ‚ Ñ‡8A;>2>5 7=0Ñ‡5=85 ?5Ñ€5<5==>9 >:Ñ€Ñƒ65=8O 8;8 fallback.
func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

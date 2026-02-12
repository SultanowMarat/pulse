package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/messenger/internal/logger"
	"github.com/messenger/internal/push"
	"gopkg.in/yaml.v3"
)

// loadEnv читает .env только вне production (в контейнере/prod конфиг только из env).
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

// CacheConfig — настройки кеша (списки чатов, избранное на клиенте).
type CacheConfig struct {
	TTLMinutes int `yaml:"ttl_minutes"`
}

// RedisConfig — Redis (OTP, rate limit, секреты сессий).
type RedisConfig struct {
	URL string `yaml:"url"`
}

// SMTPConfig — SMTP для отправки OTP (Яндекс.Почта и др.).
type SMTPConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	FromEmail string `yaml:"from_email"`
	FromName  string `yaml:"from_name"`
	UseTLS    bool   `yaml:"use_tls"`
}

// DatabaseConfig — настройки подключения к БД.
type DatabaseConfig struct {
	URL            string `yaml:"database_url"`
	MaxConnections int    `yaml:"db_max_connections"`
}

// Config содержит настройки приложения, БД и кеша.
// Приоритет: переменные окружения > YAML-файлы > значения по умолчанию.
type Config struct {
	// Сервер
	ServerAddr   string        `yaml:"server_addr"`
	ReadTimeout  time.Duration `yaml:"-"`
	WriteTimeout time.Duration `yaml:"-"`
	IdleTimeout  time.Duration `yaml:"-"`

	// База данных (загружается из config/database.yaml)
	Database DatabaseConfig `yaml:"-"`

	// Файлы
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

	// Логирование
	LogLevel string `yaml:"log_level"`

	// Кеш (загружается из config/cache.yaml)
	Cache CacheConfig `yaml:"-"`

	// Redis и SMTP (для микросервиса auth и опционально для API)
	Redis RedisConfig `yaml:"-"`
	SMTP  SMTPConfig  `yaml:"-"`

	// AuthServiceURL — URL микросервиса авторизации (для API: проверка сессий).
	AuthServiceURL string `yaml:"-"`

	// PushServiceURL — URL микросервиса пуш-уведомлений. Пустой — пуши отключены.
	PushServiceURL string `yaml:"-"`
	// PushVAPIDPublicKey — публичный VAPID-ключ для подписки в браузере (отдаётся фронту).
	PushVAPIDPublicKey string `yaml:"-"`

	// FileServiceURL — URL микросервиса файлов (upload/serve). Пустой — файлы обрабатываются в API.
	FileServiceURL string `yaml:"-"`
	// AudioServiceURL — URL микросервиса голосовых сообщений (upload/serve).
	AudioServiceURL string `yaml:"-"`
	// App status flags for maintenance/degradation banner in clients.
	AppMaintenance   bool   `yaml:"-"`
	AppReadOnly      bool   `yaml:"-"`
	AppDegradation   bool   `yaml:"-"`
	AppStatusMessage string `yaml:"-"`
}

// DatabaseURL возвращает строку подключения к БД (удобно для кода, ожидающего cfg.DatabaseURL).
func (c *Config) DatabaseURL() string { return c.Database.URL }

// DBMaxConnections возвращает максимальное число соединений в пуле.
func (c *Config) DBMaxConnections() int {
	if c.Database.MaxConnections <= 0 {
		return 20
	}
	return c.Database.MaxConnections
}

// yamlConfig — промежуточная структура для парсинга app YAML (без БД).
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

// Load загружает конфигурацию.
// Сначала подгружаются переменные из .env (если есть), затем YAML и env (env имеет приоритет).
func Load() *Config {
	loadEnv()
	// Значения по умолчанию
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

	// Загрузка конфигурации приложения: CONFIG_PATH → config/api.yaml / config/auth.yaml
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
			logger.Errorf("config: ошибка парсинга %s: %v (используются значения по умолчанию)", path, err)
		} else {
			logger.Infof("config: загружен %s", path)
		}
		break
	}

	// Загрузка конфигурации БД: DATABASE_CONFIG_PATH > config/database.yaml > config/database.yaml.example
	dbURL := "postgres://messenger:messenger_secret@localhost:5432/messenger?sslmode=disable"
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
			logger.Errorf("config: ошибка парсинга %s: %v (БД: значения по умолчанию)", path, err)
		} else {
			if dc.URL != "" {
				dbURL = dc.URL
			}
			if dc.MaxConnections > 0 {
				dbMaxConn = dc.MaxConnections
			}
			logger.Infof("config: загружен %s", path)
		}
		break
	}
	dbURL = envStr("DATABASE_URL", dbURL)
	dbMaxConn = envInt("DB_MAX_CONNECTIONS", dbMaxConn)
	if dbMaxConn <= 0 {
		dbMaxConn = 20
	}

	// Загрузка конфигурации кеша: CACHE_CONFIG_PATH > config/cache.yaml
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
			logger.Errorf("config: ошибка парсинга %s: %v (кеш: значение по умолчанию)", path, err)
		} else {
			cacheDefault = cc.TTLMinutes
			if cacheDefault <= 0 {
				cacheDefault = 10
			}
			logger.Infof("config: загружен %s", path)
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
			logger.Errorf("config: в production задайте CORS_ALLOWED_ORIGINS (явный список origins, не *)")
			// Не роняем процесс — сайт должен открываться; CORS можно задать позже
		}
		if strings.Contains(cfg.Database.URL, "messenger_secret") && strings.Contains(cfg.Database.URL, "localhost") {
			logger.Errorf("config: в production задайте DATABASE_URL (не используйте дефолт для разработки)")
			os.Exit(1)
		}
	}

	return cfg
}

// envStr возвращает значение переменной окружения или fallback.
func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt возвращает числовое значение переменной окружения или fallback.
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

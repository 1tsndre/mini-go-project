package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	pkgconstant "github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/spf13/viper"
)

type Config struct {
	App    AppConfig
	DB     DBConfig
	Redis  RedisConfig
	NSQ    NSQConfig
	JWT    JWTConfig
	Rate   RateConfig
	Upload UploadConfig
}

type AppConfig struct {
	Port            string
	Env             string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type NSQConfig struct {
	LookupdAddr string
	NsqdAddr    string
}

type JWTConfig struct {
	Secret        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type RateConfig struct {
	Public int
	Auth   int
	Login  int
}

type UploadConfig struct {
	MaxSize int64
	Dir     string
}

func (d DBConfig) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%s", d.Host, d.Port),
		Path:   d.Name,
	}

	if d.Password == "" {
		u.User = url.User(d.User)
	} else {
		u.User = url.UserPassword(d.User, d.Password)
	}

	q := u.Query()
	q.Set("sslmode", d.SSLMode)
	u.RawQuery = q.Encode()

	return u.String()
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigFile(".env")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("APP_PORT", "8080")
	v.SetDefault("APP_ENV", pkgconstant.EnvDevelopment)
	v.SetDefault("APP_READ_TIMEOUT", "15s")
	v.SetDefault("APP_WRITE_TIMEOUT", "15s")
	v.SetDefault("APP_IDLE_TIMEOUT", "60s")
	v.SetDefault("APP_SHUTDOWN_TIMEOUT", "30s")
	v.SetDefault("APP_REQUEST_TIMEOUT", "30s")
	v.SetDefault("DB_HOST", "localhost")
	v.SetDefault("DB_PORT", "5432")
	v.SetDefault("DB_USER", "postgres")
	v.SetDefault("DB_PASSWORD", "")
	v.SetDefault("DB_NAME", "mini_go_ecommerce")
	v.SetDefault("DB_SSLMODE", "disable")
	v.SetDefault("REDIS_HOST", "localhost")
	v.SetDefault("REDIS_PORT", "6379")
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("NSQ_LOOKUPD_ADDR", "localhost:4161")
	v.SetDefault("NSQD_ADDR", "localhost:4150")
	v.SetDefault("JWT_SECRET", "your-super-secret-key-change-this")
	v.SetDefault("JWT_ACCESS_EXPIRY", "15m")
	v.SetDefault("JWT_REFRESH_EXPIRY", "168h")
	v.SetDefault("RATE_LIMIT_PUBLIC", 60)
	v.SetDefault("RATE_LIMIT_AUTH", 120)
	v.SetDefault("RATE_LIMIT_LOGIN", 10)
	v.SetDefault("UPLOAD_MAX_SIZE", 5242880)
	v.SetDefault("UPLOAD_DIR", "./uploads")

	_ = v.ReadInConfig()

	accessExpiry, err := time.ParseDuration(v.GetString("JWT_ACCESS_EXPIRY"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_EXPIRY: %w", err)
	}

	refreshExpiry, err := time.ParseDuration(v.GetString("JWT_REFRESH_EXPIRY"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_EXPIRY: %w", err)
	}

	readTimeout, err := time.ParseDuration(v.GetString("APP_READ_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("invalid APP_READ_TIMEOUT: %w", err)
	}

	writeTimeout, err := time.ParseDuration(v.GetString("APP_WRITE_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("invalid APP_WRITE_TIMEOUT: %w", err)
	}

	idleTimeout, err := time.ParseDuration(v.GetString("APP_IDLE_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("invalid APP_IDLE_TIMEOUT: %w", err)
	}

	shutdownTimeout, err := time.ParseDuration(v.GetString("APP_SHUTDOWN_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("invalid APP_SHUTDOWN_TIMEOUT: %w", err)
	}

	requestTimeout, err := time.ParseDuration(v.GetString("APP_REQUEST_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("invalid APP_REQUEST_TIMEOUT: %w", err)
	}

	return &Config{
		App: AppConfig{
			Port:            v.GetString("APP_PORT"),
			Env:             v.GetString("APP_ENV"),
			ReadTimeout:     readTimeout,
			WriteTimeout:    writeTimeout,
			IdleTimeout:     idleTimeout,
			ShutdownTimeout: shutdownTimeout,
			RequestTimeout:  requestTimeout,
		},
		DB: DBConfig{
			Host:     v.GetString("DB_HOST"),
			Port:     v.GetString("DB_PORT"),
			User:     v.GetString("DB_USER"),
			Password: v.GetString("DB_PASSWORD"),
			Name:     v.GetString("DB_NAME"),
			SSLMode:  v.GetString("DB_SSLMODE"),
		},
		Redis: RedisConfig{
			Host:     v.GetString("REDIS_HOST"),
			Port:     v.GetString("REDIS_PORT"),
			Password: v.GetString("REDIS_PASSWORD"),
			DB:       v.GetInt("REDIS_DB"),
		},
		NSQ: NSQConfig{
			LookupdAddr: v.GetString("NSQ_LOOKUPD_ADDR"),
			NsqdAddr:    v.GetString("NSQD_ADDR"),
		},
		JWT: JWTConfig{
			Secret:        v.GetString("JWT_SECRET"),
			AccessExpiry:  accessExpiry,
			RefreshExpiry: refreshExpiry,
		},
		Rate: RateConfig{
			Public: v.GetInt("RATE_LIMIT_PUBLIC"),
			Auth:   v.GetInt("RATE_LIMIT_AUTH"),
			Login:  v.GetInt("RATE_LIMIT_LOGIN"),
		},
		Upload: UploadConfig{
			MaxSize: v.GetInt64("UPLOAD_MAX_SIZE"),
			Dir:     v.GetString("UPLOAD_DIR"),
		},
	}, nil
}

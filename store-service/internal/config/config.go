package config

import (
	"fmt"
	"net/url"
	"time"

	pkgconstant "github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/spf13/viper"
)

type Config struct {
	App     AppConfig
	DB      DBConfig
	Redis   RedisConfig
	NSQ     NSQConfig
	JWT     JWTConfig
	Rate    RateConfig
	Upload  UploadConfig
	Payment PaymentConfig
}

type PaymentConfig struct {
	GRPCAddr string
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
	NsqdAddr string
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
	dsn := &url.URL{
		Scheme: constant.DBScheme,
		Host:   fmt.Sprintf("%s:%s", d.Host, d.Port),
		Path:   d.Name,
	}

	if d.Password == "" {
		dsn.User = url.User(d.User)
	} else {
		dsn.User = url.UserPassword(d.User, d.Password)
	}

	query := dsn.Query()
	query.Set("sslmode", d.SSLMode)
	dsn.RawQuery = query.Encode()

	return dsn.String()
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

func Load() (*Config, error) {
	var (
		v           = viper.New()
		durationErr error
	)

	v.SetConfigFile(constant.EnvFile)
	v.AutomaticEnv()

	v.SetDefault("APP_PORT", constant.DefaultAppPort)
	v.SetDefault("APP_ENV", pkgconstant.EnvDevelopment)
	v.SetDefault("APP_READ_TIMEOUT", constant.DefaultAppReadTimeout)
	v.SetDefault("APP_WRITE_TIMEOUT", constant.DefaultAppWriteTimeout)
	v.SetDefault("APP_IDLE_TIMEOUT", constant.DefaultAppIdleTimeout)
	v.SetDefault("APP_SHUTDOWN_TIMEOUT", constant.DefaultAppShutdownTimeout)
	v.SetDefault("APP_REQUEST_TIMEOUT", constant.DefaultAppRequestTimeout)
	v.SetDefault("DB_HOST", constant.DefaultDBHost)
	v.SetDefault("DB_PORT", constant.DefaultDBPort)
	v.SetDefault("DB_USER", constant.DefaultDBUser)
	v.SetDefault("DB_PASSWORD", constant.DefaultDBPassword)
	v.SetDefault("DB_NAME", constant.DefaultDBName)
	v.SetDefault("DB_SSLMODE", constant.DefaultDBSSLMode)
	v.SetDefault("REDIS_HOST", constant.DefaultRedisHost)
	v.SetDefault("REDIS_PORT", constant.DefaultRedisPort)
	v.SetDefault("REDIS_PASSWORD", constant.DefaultRedisPassword)
	v.SetDefault("REDIS_DB", constant.DefaultRedisDB)
	v.SetDefault("NSQD_ADDR", constant.DefaultNsqdAddr)
	v.SetDefault("JWT_ACCESS_EXPIRY", constant.DefaultJWTAccessExpiry)
	v.SetDefault("JWT_REFRESH_EXPIRY", constant.DefaultJWTRefreshExpiry)
	v.SetDefault("RATE_LIMIT_PUBLIC", constant.DefaultRateLimitPublic)
	v.SetDefault("RATE_LIMIT_AUTH", constant.DefaultRateLimitAuth)
	v.SetDefault("RATE_LIMIT_LOGIN", constant.DefaultRateLimitLogin)
	v.SetDefault("UPLOAD_MAX_SIZE", constant.DefaultUploadMaxSize)
	v.SetDefault("UPLOAD_DIR", constant.DefaultUploadDir)
	v.SetDefault("PAYMENT_GRPC_ADDR", constant.DefaultPaymentGRPCAddr)

	_ = v.ReadInConfig()

	duration := func(key string) time.Duration {
		d, err := time.ParseDuration(v.GetString(key))
		if err != nil && durationErr == nil {
			durationErr = fmt.Errorf(constant.MsgInvalidDuration, key, err)
		}
		return d
	}

	cfg := &Config{
		App: AppConfig{
			Port:            v.GetString("APP_PORT"),
			Env:             v.GetString("APP_ENV"),
			ReadTimeout:     duration("APP_READ_TIMEOUT"),
			WriteTimeout:    duration("APP_WRITE_TIMEOUT"),
			IdleTimeout:     duration("APP_IDLE_TIMEOUT"),
			ShutdownTimeout: duration("APP_SHUTDOWN_TIMEOUT"),
			RequestTimeout:  duration("APP_REQUEST_TIMEOUT"),
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
			NsqdAddr: v.GetString("NSQD_ADDR"),
		},
		JWT: JWTConfig{
			Secret:        v.GetString("JWT_SECRET"),
			AccessExpiry:  duration("JWT_ACCESS_EXPIRY"),
			RefreshExpiry: duration("JWT_REFRESH_EXPIRY"),
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
		Payment: PaymentConfig{
			GRPCAddr: v.GetString("PAYMENT_GRPC_ADDR"),
		},
	}

	if durationErr != nil {
		return nil, durationErr
	}

	// Otherwise a timed-out request could not be sent its 504.
	if cfg.App.WriteTimeout <= cfg.App.RequestTimeout {
		return nil, fmt.Errorf(constant.MsgWriteTimeoutTooShort, cfg.App.WriteTimeout, cfg.App.RequestTimeout)
	}
	return cfg, nil
}

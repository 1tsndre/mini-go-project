package config

import (
	"strings"

	pkgconstant "github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/spf13/viper"
)

type Config struct {
	App     AppConfig
	NSQ     NSQConfig
	Payment PaymentConfig
}

type AppConfig struct {
	Env string
}

type NSQConfig struct {
	LookupdAddr string
	NsqdAddr    string
}

type PaymentConfig struct {
	GRPCPort string
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigFile(".env")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("APP_ENV", pkgconstant.EnvProduction)
	v.SetDefault("NSQ_LOOKUPD_ADDR", "localhost:4161")
	v.SetDefault("NSQD_ADDR", "localhost:4150")
	v.SetDefault("PAYMENT_GRPC_PORT", "50051")

	_ = v.ReadInConfig()

	return &Config{
		App: AppConfig{
			Env: v.GetString("APP_ENV"),
		},
		NSQ: NSQConfig{
			LookupdAddr: v.GetString("NSQ_LOOKUPD_ADDR"),
			NsqdAddr:    v.GetString("NSQD_ADDR"),
		},
		Payment: PaymentConfig{
			GRPCPort: v.GetString("PAYMENT_GRPC_PORT"),
		},
	}, nil
}

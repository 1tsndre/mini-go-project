package config

import (
	"github.com/1tsndre/mini-go-project/payment-service/internal/constant"
	pkgconstant "github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/spf13/viper"
)

type Config struct {
	Env      string
	NsqdAddr string
	GRPCPort string
}

func Load() Config {
	v := viper.New()

	v.SetConfigFile(constant.EnvFile)
	v.AutomaticEnv()

	v.SetDefault("APP_ENV", pkgconstant.EnvDevelopment)
	v.SetDefault("NSQD_ADDR", constant.DefaultNsqdAddr)
	v.SetDefault("PAYMENT_GRPC_PORT", constant.DefaultGRPCPort)

	_ = v.ReadInConfig()

	return Config{
		Env:      v.GetString("APP_ENV"),
		NsqdAddr: v.GetString("NSQD_ADDR"),
		GRPCPort: v.GetString("PAYMENT_GRPC_PORT"),
	}
}

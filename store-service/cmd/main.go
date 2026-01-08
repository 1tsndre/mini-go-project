package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/config"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases/postgres"
	"github.com/1tsndre/mini-go-project/store-service/internal/router"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(constant.LogConfigLoadFailed, err)
	}

	logger.Init(cfg.App.Env)

	ctx := context.Background()

	db, err := postgres.NewPostgresDB(cfg.DB.DSN(), cfg.App.Env)
	if err != nil {
		logger.Fatal(ctx, constant.LogDBConnectFailed, err)
	}
	logger.Info(ctx, constant.LogDBConnected)

	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal(ctx, constant.LogRedisConnectFailed, err)
	}
	logger.Info(ctx, constant.LogRedisConnected)

	maxBodyBytes := cfg.Upload.MaxSize + constant.MultipartOverhead
	httpHandler := router.NewRouter(maxBodyBytes, cfg.App.RequestTimeout)

	server := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      httpHandler,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		IdleTimeout:  cfg.App.IdleTimeout,
	}

	go func() {
		logger.Info(ctx, constant.LogServerStarting, logger.Fields{
			"port": cfg.App.Port,
		})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx, constant.LogServerFailed, err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, constant.LogServerShuttingDown)

	shutdownCtx, cancel := context.WithTimeout(ctx, cfg.App.ShutdownTimeout)
	defer cancel()

	// Drain HTTP first: in-flight requests still need the connections below.
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, constant.LogServerForcedShutdown, err)
	}

	redisClient.Close()
	db.Close()

	logger.Info(ctx, constant.LogServerStopped)
}

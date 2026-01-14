package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	pkgjwt "github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/pkg/upload"
	"github.com/1tsndre/mini-go-project/store-service/internal/config"
	"github.com/1tsndre/mini-go-project/store-service/internal/handler"
	"github.com/1tsndre/mini-go-project/store-service/internal/nsq"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	rediscache "github.com/1tsndre/mini-go-project/store-service/internal/repository/caches/redis"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases/postgres"
	"github.com/1tsndre/mini-go-project/store-service/internal/router"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	goredis "github.com/redis/go-redis/v9"

	"github.com/go-redsync/redsync/v4"
	redsyncredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	gonsq "github.com/nsqio/go-nsq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.JWT.Secret == "" {
		log.Fatal("JWT_SECRET must be set")
	}

	logger.Init(cfg.App.Env)

	ctx := context.Background()

	db, err := postgres.NewPostgresDB(cfg.DB.DSN(), cfg.App.Env)
	if err != nil {
		logger.Fatal(ctx, "failed to connect to database", err)
	}
	logger.Info(ctx, "connected to database")

	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal(ctx, "failed to connect to redis", err)
	}
	logger.Info(ctx, "connected to redis")

	pool := redsyncredis.NewPool(redisClient)
	rs := redsync.New(pool)

	nsqProducer, err := gonsq.NewProducer(cfg.NSQ.NsqdAddr, gonsq.NewConfig())
	if err != nil {
		logger.Fatal(ctx, "failed to create NSQ producer", err)
	}
	logger.Info(ctx, "connected to NSQ")

	cache := rediscache.NewRedisCache(redisClient)

	userRepo := repository.NewUserRepository(db)
	storeRepo := repository.NewStoreRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	productRepo := repository.NewProductRepository(db, cache)
	cartRepo := repository.NewCartRepository(db, cache)
	orderRepo := repository.NewOrderRepository(db)
	reviewRepo := repository.NewReviewRepository(db)

	jwtManager := pkgjwt.NewJWTManager(cfg.JWT.Secret, cfg.JWT.AccessExpiry, cfg.JWT.RefreshExpiry)

	authService := service.NewAuthService(userRepo, jwtManager)
	storeService := service.NewStoreService(storeRepo, userRepo)
	categoryService := service.NewCategoryService(categoryRepo)
	productService := service.NewProductService(productRepo, storeRepo)
	cartService := service.NewCartService(cartRepo, productRepo, rs)
	orderService := service.NewOrderService(orderRepo, cartRepo, productRepo, storeRepo, rs, nsqProducer)
	reviewService := service.NewReviewService(reviewRepo)

	uploader := upload.NewUploader(cfg.Upload.Dir, cfg.Upload.MaxSize)

	handlers := router.Handlers{
		Auth:     handler.NewAuthHandler(authService),
		Store:    handler.NewStoreHandler(storeService, uploader),
		Category: handler.NewCategoryHandler(categoryService),
		Product:  handler.NewProductHandler(productService, uploader),
		Cart:     handler.NewCartHandler(cartService),
		Order:    handler.NewOrderHandler(orderService),
		Review:   handler.NewReviewHandler(reviewService),
	}

	paymentConsumer := nsq.NewPaymentResultConsumer(orderService)
	if err := paymentConsumer.Start(cfg.NSQ.LookupdAddr); err != nil {
		logger.Warn(ctx, "failed to start NSQ consumer, payment callbacks won't work", map[string]interface{}{
			"error": err.Error(),
		})
	}

	handler := router.NewRouter(handlers, jwtManager, redisClient, cfg.Upload.Dir, cfg.App.RequestTimeout, cfg.Rate)

	server := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      handler,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		IdleTimeout:  cfg.App.IdleTimeout,
	}

	go func() {
		logger.Info(ctx, "server starting", map[string]interface{}{
			"port": cfg.App.Port,
		})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx, "server failed", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(ctx, cfg.App.ShutdownTimeout)
	defer cancel()

	paymentConsumer.Stop()
	nsqProducer.Stop()
	redisClient.Close()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatal(ctx, "server forced to shutdown", err)
	}

	logger.Info(ctx, "server stopped")
}

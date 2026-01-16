package router

import (
	"net/http"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/jwt"
	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/config"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/handler"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/redis/go-redis/v9"
)

type Handlers struct {
	Auth     *handler.AuthHandler
	Store    *handler.StoreHandler
	Category *handler.CategoryHandler
	Product  *handler.ProductHandler
	Cart     *handler.CartHandler
	Order    *handler.OrderHandler
	Review   *handler.ReviewHandler
}

func NewRouter(
	handlers Handlers,
	jwtManager *jwt.JWTManager,
	redisClient *redis.Client,
	uploadDir string,
	requestTimeout time.Duration,
	rateCfg config.RateConfig,
) http.Handler {
	mux := http.NewServeMux()

	rateLimiter := middleware.NewRateLimiter(redisClient)
	authMw := middleware.Auth(jwtManager)
	sellerMw := middleware.RequireRole(constant.RoleSeller)
	buyerMw := middleware.RequireRole(constant.RoleBuyer)
	adminMw := middleware.RequireRole(constant.RoleAdmin)
	loginRate := rateLimiter.Limit(rateCfg.Login, time.Minute, constant.RateLimitKeyLogin)
	publicRate := rateLimiter.Limit(rateCfg.Public, time.Minute, constant.RateLimitKeyPublic)
	authRate := rateLimiter.Limit(rateCfg.Auth, time.Minute, constant.RateLimitKeyAuth)

	// 404 catch-all for routes not matched by any other pattern
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		meta := middleware.BuildMeta(r)
		response.ErrorResponse(w, http.StatusNotFound, meta,
			response.NewError(constant.ErrCodeNotFound, "not found"),
		)
	})

	// Static files
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir("./docs"))))

	// Auth routes
	mux.Handle("POST /api/v1/auth/register", middleware.Chain(http.HandlerFunc(handlers.Auth.Register), loginRate, publicRate))
	mux.Handle("POST /api/v1/auth/login", middleware.Chain(http.HandlerFunc(handlers.Auth.Login), loginRate, publicRate))
	mux.Handle("POST /api/v1/auth/refresh", middleware.Chain(http.HandlerFunc(handlers.Auth.Refresh), authRate))

	// Store routes
	mux.Handle("POST /api/v1/stores", middleware.Chain(http.HandlerFunc(handlers.Store.CreateStore), authMw, buyerMw, authRate))
	mux.Handle("GET /api/v1/stores/{id}", middleware.Chain(http.HandlerFunc(handlers.Store.GetStore), publicRate))
	mux.Handle("PUT /api/v1/stores/{id}", middleware.Chain(http.HandlerFunc(handlers.Store.UpdateStore), authMw, sellerMw, authRate))
	mux.Handle("POST /api/v1/stores/{id}/logo", middleware.Chain(http.HandlerFunc(handlers.Store.UploadLogo), authMw, sellerMw, authRate))

	// Category routes
	mux.Handle("POST /api/v1/categories", middleware.Chain(http.HandlerFunc(handlers.Category.CreateCategory), authMw, adminMw, authRate))
	mux.Handle("GET /api/v1/categories", middleware.Chain(http.HandlerFunc(handlers.Category.GetCategories), publicRate))
	mux.Handle("PUT /api/v1/categories/{id}", middleware.Chain(http.HandlerFunc(handlers.Category.UpdateCategory), authMw, adminMw, authRate))
	mux.Handle("DELETE /api/v1/categories/{id}", middleware.Chain(http.HandlerFunc(handlers.Category.DeleteCategory), authMw, adminMw, authRate))

	// Product routes
	mux.Handle("POST /api/v1/products", middleware.Chain(http.HandlerFunc(handlers.Product.CreateProduct), authMw, sellerMw, authRate))
	mux.Handle("GET /api/v1/products", middleware.Chain(http.HandlerFunc(handlers.Product.GetProducts), publicRate))
	mux.Handle("GET /api/v1/products/{id}", middleware.Chain(http.HandlerFunc(handlers.Product.GetProduct), publicRate))
	mux.Handle("PUT /api/v1/products/{id}", middleware.Chain(http.HandlerFunc(handlers.Product.UpdateProduct), authMw, sellerMw, authRate))
	mux.Handle("DELETE /api/v1/products/{id}", middleware.Chain(http.HandlerFunc(handlers.Product.DeleteProduct), authMw, sellerMw, authRate))
	mux.Handle("POST /api/v1/products/{id}/image", middleware.Chain(http.HandlerFunc(handlers.Product.UploadImage), authMw, sellerMw, authRate))

	// Review routes
	mux.Handle("POST /api/v1/products/{id}/reviews", middleware.Chain(http.HandlerFunc(handlers.Review.CreateReview), authMw, buyerMw, authRate))
	mux.Handle("GET /api/v1/products/{id}/reviews", middleware.Chain(http.HandlerFunc(handlers.Review.GetProductReviews), publicRate))

	// Cart routes
	mux.Handle("GET /api/v1/cart", middleware.Chain(http.HandlerFunc(handlers.Cart.GetCart), authMw, buyerMw, authRate))
	mux.Handle("POST /api/v1/cart/items", middleware.Chain(http.HandlerFunc(handlers.Cart.AddItem), authMw, buyerMw, authRate))
	mux.Handle("PUT /api/v1/cart/items/{product_id}", middleware.Chain(http.HandlerFunc(handlers.Cart.UpdateItem), authMw, buyerMw, authRate))
	mux.Handle("DELETE /api/v1/cart/items/{product_id}", middleware.Chain(http.HandlerFunc(handlers.Cart.RemoveItem), authMw, buyerMw, authRate))

	// Order routes (buyer)
	mux.Handle("POST /api/v1/orders", middleware.Chain(http.HandlerFunc(handlers.Order.Checkout), authMw, buyerMw, authRate))
	mux.Handle("GET /api/v1/orders", middleware.Chain(http.HandlerFunc(handlers.Order.GetOrders), authMw, buyerMw, authRate))
	mux.Handle("GET /api/v1/orders/{id}", middleware.Chain(http.HandlerFunc(handlers.Order.GetOrder), authMw, buyerMw, authRate))
	mux.Handle("PUT /api/v1/orders/{id}/cancel", middleware.Chain(http.HandlerFunc(handlers.Order.CancelOrder), authMw, buyerMw, authRate))

	// Order routes (seller)
	mux.Handle("GET /api/v1/seller/orders", middleware.Chain(http.HandlerFunc(handlers.Order.GetSellerOrders), authMw, sellerMw, authRate))
	mux.Handle("PUT /api/v1/orders/{id}/status", middleware.Chain(http.HandlerFunc(handlers.Order.UpdateOrderStatus), authMw, sellerMw, authRate))

	return middleware.Chain(mux,
		middleware.Recovery,
		middleware.Timeout(requestTimeout),
		middleware.Logging,
		middleware.RequestID,
		middleware.MethodNotAllowed,
	)
}

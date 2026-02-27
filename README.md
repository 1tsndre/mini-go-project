# Mini Go E-Commerce API

An E-Commerce REST API built with Go, following Clean Architecture principles. Features a monorepo multi-service setup with asynchronous messaging and inter-service communication via gRPC.

## Tech Stack

| Technology | Purpose |
|------------|---------|
| **Go** (net/http) | HTTP server and routing (Go 1.22+) |
| **PostgreSQL** | Primary data store |
| **Redis** | Cart storage, product caching, distributed locking, rate limiting |
| **NSQ** | Asynchronous order-to-payment pipeline |
| **gRPC** | Inter-service communication (payment processing) |
| **GORM** | ORM and database abstraction |
| **JWT** | Authentication with access/refresh token pair |
| **Viper** | Configuration management |
| **zerolog** | Structured logging with request tracing |
| **redsync** | Redis-based distributed mutex for stock management |

## Architecture

```
┌───────────────────────────────────────────────────────┐
│                    store-service                      │
│                                                       │
│  Handler ──▶ Service ──▶ Repository                   │
│     │            │            ├── databases/postgres  │
│     │            │            └── caches/redis        │
│  Middleware      │                                    │
│  (auth, rate     │         NSQ Producer               │
│   limit, log)    │              │                     │
└──────────────────┼──────────────┼─────────────────────┘
                   │              │
                   │              ▼
                   │     ┌────────────────┐
                   │     │      NSQ       │
                   │     └───────┬────────┘
                   │             │
                   │             ▼
              ┌────┴─────────────────────────┐
              │      payment-service         │
              │                              │
              │  gRPC Server ◀── Service     │
              │       NSQ Consumer/Producer  │
              └──────────────────────────────┘
```

Each layer communicates via interfaces, making the codebase testable and loosely coupled. Dependencies are injected manually in `main.go`.

## Features

- **Auth** — JWT access/refresh tokens, role-based access control (Admin, Buyer, Seller)
- **Products** — Full CRUD, full-text search, filter by category/price, image upload
- **Cart** — Redis-first with PostgreSQL fallback, persists across sessions
- **Orders** — Checkout with distributed lock for stock consistency, status flow: `pending → paid → processing → shipping → shipped → completed`, cancellation up to `processing`
- **Payment Pipeline** — Async via NSQ: order created → payment processed (mock) → status updated
- **Reviews** — One review per purchased product, rating 1–5 with optional comment
- **Rate Limiting** — Sliding window using Redis Sorted Sets
- **Observability** — Structured logging (zerolog) with request ID propagation, graceful shutdown

## Project Structure

<details>
<summary>Click to expand</summary>

```
mini-go-project/
├── store-service/                 # Main REST API service
│   ├── cmd/main.go                # Entry point, dependency injection
│   └── internal/
│       ├── config/                # Viper-based configuration
│       ├── constant/              # Redis keys, roles, statuses, error codes, NSQ topics, rate limit key types
│       ├── model/                 # Entities and DTOs
│       ├── repository/            # Data access layer
│       │   ├── caches/            # Cache interface + Redis implementation
│       │   └── databases/         # Database interface + PostgreSQL implementation
│       ├── service/               # Business logic layer
│       ├── handler/               # HTTP handlers
│       ├── middleware/            # request_id, logging, recovery, auth, rate_limiter, timeout, json_errors
│       ├── router/                # Route registration
│       ├── nsq/                   # NSQ consumer (payment results)
│       └── mocks/                 # Generated mocks for testing
│
├── payment-service/               # gRPC + NSQ payment processor
│   ├── cmd/main.go
│   └── internal/
│       ├── config/
│       ├── handler/               # gRPC server implementation
│       ├── service/               # Mock payment logic
│       └── nsq/                   # NSQ consumer/producer
│
├── proto/payment/                 # gRPC protobuf definitions
├── pkg/                           # Shared packages (logger, jwt, response, upload)
├── migrations/                    # SQL migration files
├── docs/                          # Static OpenAPI spec
└── .env.example
```

</details>

## Getting Started

### Prerequisites

- Go 1.22+
- PostgreSQL 16+
- Redis 7+
- NSQ
- [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate)

### Setup

**1. Clone and configure**

```bash
git clone https://github.com/1tsndre/mini-go-project.git
cd mini-go-project
cp .env.example .env
# Fill in your PostgreSQL, Redis, and JWT credentials
```

**2. Run NSQ**

Download the NSQ binary from https://nsq.io/deployment/installing.html, then run in separate terminals:

```bash
nsqlookupd
nsqd --lookupd-tcp-address=localhost:4160
```

**3. Run database migrations**

```bash
migrate -path migrations \
  -database "postgresql://postgres:yourpassword@localhost:5432/mini_go_ecommerce?sslmode=disable" \
  up
```

**4. Build and run**

```bash
go build -o bin/store-service ./store-service/cmd
go build -o bin/payment-service ./payment-service/cmd

./bin/payment-service
./bin/store-service
```

API available at `http://localhost:8080`. OpenAPI spec at `http://localhost:8080/docs/swagger.json`.

### Run without building (development)

```bash
go run payment-service/cmd/main.go
go run store-service/cmd/main.go
```

## API Endpoints

<details>
<summary>Click to expand</summary>

### Health
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/health` | Service health check | - |

### Auth
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/api/v1/auth/register` | Register new user | - |
| POST | `/api/v1/auth/login` | Login | - |
| POST | `/api/v1/auth/refresh` | Refresh token | Bearer |

### Store
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/api/v1/stores` | Create store (become seller) | Buyer |
| GET | `/api/v1/stores/:id` | Get store details | - |
| PUT | `/api/v1/stores/:id` | Update store | Seller |
| POST | `/api/v1/stores/:id/logo` | Upload store logo | Seller |

### Category
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/api/v1/categories` | Create category | Admin |
| GET | `/api/v1/categories` | List categories | - |
| PUT | `/api/v1/categories/:id` | Update category | Admin |
| DELETE | `/api/v1/categories/:id` | Delete category | Admin |

### Product
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/api/v1/products` | Create product | Seller |
| GET | `/api/v1/products` | List/search/filter products | - |
| GET | `/api/v1/products/:id` | Get product detail | - |
| PUT | `/api/v1/products/:id` | Update product | Seller |
| DELETE | `/api/v1/products/:id` | Delete product | Seller |
| POST | `/api/v1/products/:id/image` | Upload product image | Seller |

### Review
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/api/v1/products/:id/reviews` | Create review | Buyer |
| GET | `/api/v1/products/:id/reviews` | List reviews | - |

### Cart
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/api/v1/cart` | Get cart | Buyer |
| POST | `/api/v1/cart/items` | Add item to cart | Buyer |
| PUT | `/api/v1/cart/items/:product_id` | Update item quantity | Buyer |
| DELETE | `/api/v1/cart/items/:product_id` | Remove item from cart | Buyer |

### Order
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/api/v1/orders` | Checkout (create order) | Buyer |
| GET | `/api/v1/orders` | List buyer orders | Buyer |
| GET | `/api/v1/orders/:id` | Get order detail | Buyer |
| PUT | `/api/v1/orders/:id/cancel` | Cancel order | Buyer |
| GET | `/api/v1/seller/orders` | List seller orders | Seller |
| PUT | `/api/v1/orders/:id/status` | Update order status | Seller |

</details>

## Response Format

<details>
<summary>Click to expand</summary>

```json
// Success
{
  "data": { ... },
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2026-02-20T10:00:00Z",
    "pagination": { "current_page": 1, "per_page": 10, "total_items": 100, "total_pages": 10 }
  }
}

// Error
{
  "errors": [{ "code": "VALIDATION_ERROR", "field": "email", "message": "is required" }],
  "meta": { "request_id": "550e8400-e29b-41d4-a716-446655440000", "timestamp": "2026-02-20T10:00:00Z" }
}
```

</details>

## Environment Variables

<details>
<summary>Click to expand</summary>

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_PORT` | 8080 | Application port |
| `APP_ENV` | development | Environment |
| `APP_REQUEST_TIMEOUT` | 30s | Per-request timeout |
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 5432 | PostgreSQL port |
| `DB_USER` | postgres | PostgreSQL user |
| `DB_PASSWORD` | - | PostgreSQL password |
| `DB_NAME` | mini_go_ecommerce | Database name |
| `DB_SSLMODE` | disable | PostgreSQL SSL mode |
| `REDIS_HOST` | localhost | Redis host |
| `REDIS_PORT` | 6379 | Redis port |
| `REDIS_PASSWORD` | - | Redis password |
| `NSQ_LOOKUPD_ADDR` | localhost:4161 | NSQ Lookupd address |
| `NSQD_ADDR` | localhost:4150 | NSQd address |
| `JWT_SECRET` | - | JWT signing secret |
| `JWT_ACCESS_EXPIRY` | 15m | Access token expiry |
| `JWT_REFRESH_EXPIRY` | 168h | Refresh token expiry |
| `RATE_LIMIT_PUBLIC` | 60 | Req/min for public endpoints |
| `RATE_LIMIT_AUTH` | 120 | Req/min for authenticated endpoints |
| `RATE_LIMIT_LOGIN` | 10 | Req/min for login endpoint |
| `UPLOAD_MAX_SIZE` | 5242880 | Max upload size (bytes) |
| `UPLOAD_DIR` | ./uploads | Upload directory |
| `PAYMENT_GRPC_PORT` | 50051 | Payment service gRPC port |

</details>

## Testing

```bash
go test ./... -v
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

## License

This project is a personal portfolio project.

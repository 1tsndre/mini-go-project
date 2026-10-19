# Mini Go E-Commerce API

An E-Commerce REST API built with Go, following Clean Architecture principles. Features a monorepo multi-service setup with asynchronous messaging and inter-service communication via gRPC.

## Tech Stack

| Technology | Purpose |
|------------|---------|
| **Go** (net/http) | HTTP server and routing (Go 1.22+) |
| **PostgreSQL** | Primary data store |
| **Redis** | Cart storage, product caching, distributed locking, rate limiting |
| **NSQ** | Asynchronous order-to-payment pipeline |
| **gRPC** | Synchronous payment-status query (store-service → payment-service) |
| **sqlx** | Plain SQL queries scanned into structs (no ORM) |
| **pgx** | PostgreSQL driver |
| **golang-migrate** | Versioned SQL schema migrations |
| **JWT** | Authentication with access/refresh token pair |
| **Viper** | Configuration management |
| **zerolog** | Structured logging with request tracing |
| **redsync** | Redis-based distributed mutex for concurrent cart updates |

## Architecture

```
┌───────────────────────────────────────────────────────┐
│                    store-service                      │
│                                                       │
│  Handler ──▶ Service ──▶ Repository                   │
│     │            │            ├── databases/postgres  │
│     │            │            └── caches/redis        │
│  Middleware      │                                    │
│  (auth, rate     │     NSQ Producer   NSQ Consumer    │
│   limit, log)    │          │              ▲          │
└──────────────────┼──────────┼──────────────┼──────────┘
                   │ gRPC     │ order.       │ payment.success /
                   │          │ created      │ payment.failed
                   │          ▼              │
                   │     ┌─────────────────────────┐
                   │     │           NSQ           │
                   │     └─────────────────────────┘
                   │          │              ▲
                   ▼          ▼              │
              ┌─────────────────────────────────────┐
              │          payment-service            │
              │                                     │
              │  gRPC Server ◀── Service            │
              │  NSQ Consumer ──▶ NSQ Producer      │
              └─────────────────────────────────────┘
```

Each layer communicates via interfaces, making the codebase testable and loosely coupled. Dependencies are injected manually in `main.go`.

Repositories run plain SQL through sqlx; there is no ORM. The schema is defined only by the SQL files in `migrations/`, applied with golang-migrate, so the service never creates or alters tables itself. Missing rows and constraint violations are translated into `repository.ErrNotFound`, `ErrDuplicateKey` and `ErrForeignKeyViolation`, so services never depend on the database driver.

## Features

- **Auth** — JWT access/refresh tokens, role-based access control (Admin, Buyer, Seller). Refresh tokens are rotated: each one works once, its session is stored in Redis, and logout revokes it
- **Products** — Full CRUD, search by name and description, filter by category/price, image upload
- **Cart** — Redis-first with PostgreSQL fallback, persists across sessions, always shows current product prices (the price checkout charges)
- **Orders** — Checkout reserves stock, creates one order per store and empties the cart in a single database transaction, so stock can never go negative and a cart cannot be ordered twice. Status flow: `pending → paid → processing → shipping → shipped → completed`; cancelling is possible up to `processing`, returns the stock and refunds the payment (mock)
- **Payment Pipeline** — Async via NSQ: order created → payment processed (mock) → status updated. NSQ redelivers a message until it is handled, checkout cancels orders it cannot publish and gives the cart back, and orders still pending are republished when the store-service starts
- **Reviews** — One review per purchased product, rating 1–5 with optional comment
- **Rate Limiting** — Sliding window using Redis Sorted Sets
- **Observability** — Structured logging (zerolog) with request ID propagation, SQL query logging in development, graceful shutdown

## Project Structure

<details>
<summary>Click to expand</summary>

```
mini-go-project/
├── store-service/                 # Main REST API service
│   ├── cmd/main.go                # Entry point, dependency injection
│   └── internal/
│       ├── config/                # Viper-based configuration
│       ├── constant/              # All fixed values: messages, log text, Redis keys, statuses, limits, NSQ topics
│       ├── model/                 # Entities and DTOs
│       ├── repository/            # Data access layer (plain SQL via sqlx)
│       │   ├── caches/            # Cache interface + Redis implementation
│       │   └── databases/         # PostgreSQL connection (pgx)
│       ├── service/               # Business logic layer
│       ├── handler/               # HTTP handlers
│       ├── middleware/            # request_id, logging, body_limit, recovery, auth, rate_limiter, timeout
│       ├── router/                # Route registration
│       ├── pagination/            # Applies the page and per_page limits
│       ├── nsq/                   # NSQ consumer (payment results)
│       ├── grpc/                  # gRPC client (payment status)
│       └── mocks/                 # Generated mocks for testing
│
├── payment-service/               # gRPC + NSQ payment processor
│   ├── cmd/main.go
│   └── internal/
│       ├── config/
│       ├── constant/              # NSQ topics, payment statuses, log text, mock payment settings
│       ├── handler/               # gRPC server implementation
│       ├── service/               # Mock payment logic
│       └── nsq/                   # NSQ consumer/producer
│
├── proto/payment/                 # gRPC protobuf definitions
├── pkg/                           # Shared packages (logger, jwt, response, upload, constant)
├── migrations/                    # SQL schema migrations (golang-migrate)
├── docs/                          # Static OpenAPI spec
└── .env.example
```

</details>

## Getting Started

### Run with Docker

Needs only Docker and Docker Compose. Builds both services, runs the database migrations, then starts everything together with PostgreSQL, Redis and NSQ:

```bash
git clone https://github.com/1tsndre/mini-go-project.git
cd mini-go-project
docker compose up -d --build
docker compose logs -f store-service   # tail logs
docker compose down                    # stop everything
```

API available at `http://localhost:8080`, OpenAPI spec at `http://localhost:8080/docs/swagger.json`, NSQ admin at `http://localhost:4171`.

### Run locally

Needs Go 1.25.6+, PostgreSQL 16+, Redis 7+, [NSQ](https://nsq.io/deployment/installing.html) and golang-migrate:

```bash
go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
```

```bash
cp .env.example .env                   # fill in the PostgreSQL password and JWT_SECRET
nsqd --mem-queue-size=0                # in its own terminal; keeps queued messages on disk
createdb -U postgres mini_go_ecommerce # once

# on a new database, and again whenever migrations/ gets new files
migrate -path migrations \
  -database "postgresql://postgres:yourpassword@localhost:5432/mini_go_ecommerce?sslmode=disable" \
  up

go run payment-service/cmd/main.go     # in its own terminal
go run store-service/cmd/main.go
```

## API Endpoints

Request and response formats are in [docs/swagger.json](docs/swagger.json).

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
| POST | `/api/v1/auth/refresh` | Refresh token (refresh token in request body; it is revoked and replaced) | - |
| POST | `/api/v1/auth/logout` | Log out (revokes the refresh token in the request body) | - |

### Store
| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/api/v1/stores` | Create store (become seller) | Buyer |
| GET | `/api/v1/stores/me` | Get my store (404 until the user opens one) | Any role |
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
| POST | `/api/v1/orders` | Checkout (creates one order per store) | Buyer |
| GET | `/api/v1/orders` | List my orders | Any role |
| GET | `/api/v1/orders/:id` | Get order detail | Any role |
| PUT | `/api/v1/orders/:id/cancel` | Cancel order | Any role |
| GET | `/api/v1/orders/:id/payment` | Get payment status (via gRPC to payment-service) | Any role |
| GET | `/api/v1/seller/orders` | List seller orders | Seller |
| PUT | `/api/v1/orders/:id/status` | Update order status | Seller |

</details>

## Environment Variables

<details>
<summary>Click to expand</summary>

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_PORT` | 8080 | Application port |
| `APP_ENV` | development | Environment |
| `APP_REQUEST_TIMEOUT` | 30s | Per-request timeout (responds 504 when exceeded) |
| `APP_READ_TIMEOUT` | 15s | Max time to read a request, including the body |
| `APP_WRITE_TIMEOUT` | 35s | Max time to write a response; must be greater than `APP_REQUEST_TIMEOUT` |
| `APP_IDLE_TIMEOUT` | 60s | Keep-alive idle timeout |
| `APP_SHUTDOWN_TIMEOUT` | 30s | Grace period for in-flight requests on shutdown |
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 5432 | PostgreSQL port |
| `DB_USER` | postgres | PostgreSQL user |
| `DB_PASSWORD` | - | PostgreSQL password |
| `DB_NAME` | mini_go_ecommerce | Database name |
| `DB_SSLMODE` | disable | PostgreSQL SSL mode |
| `REDIS_HOST` | localhost | Redis host |
| `REDIS_PORT` | 6379 | Redis port |
| `REDIS_PASSWORD` | - | Redis password |
| `REDIS_DB` | 0 | Redis database number |
| `NSQD_ADDR` | localhost:4150 | NSQd address |
| `JWT_SECRET` | - | JWT signing secret |
| `JWT_ACCESS_EXPIRY` | 15m | Access token expiry |
| `JWT_REFRESH_EXPIRY` | 168h | Refresh token expiry |
| `RATE_LIMIT_PUBLIC` | 60 | Req/min for public endpoints |
| `RATE_LIMIT_AUTH` | 120 | Req/min for authenticated endpoints |
| `RATE_LIMIT_LOGIN` | 10 | Req/min for login endpoint |
| `UPLOAD_MAX_SIZE` | 5242880 | Max upload size (bytes) |
| `UPLOAD_DIR` | ./uploads | Upload directory |
| `PAYMENT_GRPC_PORT` | 50051 | Payment service gRPC port (payment-service server) |
| `PAYMENT_GRPC_ADDR` | localhost:50051 | Payment service gRPC address (store-service client) |

</details>

## Testing

```bash
go test ./... -v
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

Without a database, the repository tests still check the selected columns and the models' `db` tags against `migrations/`. With `TEST_DATABASE_URL` set, they also run every query against a real PostgreSQL (those tests are skipped otherwise). They migrate a temporary schema and drop it afterwards, so the database's own data is left untouched — for example, with the Docker Compose database:

```bash
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mini_go_ecommerce?sslmode=disable" \
  go test ./store-service/internal/repository/ -v
```

CI runs them on every push, against a PostgreSQL service container.

## License

This project is a personal portfolio project.

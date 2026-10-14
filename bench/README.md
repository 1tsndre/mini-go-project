# Benchmark

A [k6](https://k6.io) load test of the store-service. This project has four ports that serve the same API, byte for byte: [Java](https://github.com/1tsndre/mini-java-project) (Spring Boot), [Python](https://github.com/1tsndre/mini-python-project) (Starlette), and [Express and NestJS](https://github.com/1tsndre/mini-nodeJS-project). The script only uses endpoints all of them have, so it measures the five implementations doing the same work.

The ports do not have this project's newest features yet: logout and refresh-token rotation, the transactional outbox and the `Idempotency-Key` header. The ports ignore that header, and their login does not store a session in Redis, so the Go figures for `auth` and `checkout` include a little work the others skip.

## Scenarios

| Scenario | Requests per iteration | What it stresses |
|----------|------------------------|------------------|
| `browse` | `GET /products?page=1&per_page=20`, `GET /products/{id}` | Public reads: SQL listing with a count, Redis-cached product details, JSON encoding |
| `auth` | `POST /auth/login` | bcrypt (cost 10) |
| `checkout` | `POST /cart/items`, `POST /orders` with an `Idempotency-Key` | The write path: the Redis cart lock, the checkout transaction with its stock reservation (in Go also its outbox row and the idempotency record) |

Each run first creates its own category, store, 20 products and one buyer per virtual user (VU), so runs never share carts and can repeat against the same database. Only the seeded admin account is needed.

## Running

Install k6 (any recent version; written against v2.3), then start the services of one implementation. The rate limits must be raised, or k6 measures the 429s:

```bash
go run payment-service/cmd/main.go   # checkout publishes orders to it through NSQ
RATE_LIMIT_LOGIN=1000000 RATE_LIMIT_PUBLIC=1000000 RATE_LIMIT_AUTH=1000000 go run store-service/cmd/main.go
```

Then, from this repository:

```bash
bash bench/run.sh go http://localhost:8080             # all three scenarios, 20 VUs for 60 s each
VUS=50 DURATION=2m bash bench/run.sh go http://localhost:8080 browse
```

Stop the services, start the next implementation the same way (each port's README shows how), and run `bench/run.sh` again with its name: `java`, `python`, `express` or `nestjs`. The summaries land in `bench/results/`; this prints them as a table:

```bash
python3 bench/report.py
```

Requests per second count the scenario's requests over its duration; the requests `setup()` makes are left out of every figure.

## Fair comparisons

- Run every implementation on the same machine, against the same PostgreSQL, Redis and NSQ, with nothing else busy. A phone or a VM with throttled CPUs gives numbers that say more about the host than the code.
- Use each implementation's production settings: `APP_ENV=production` (JSON logs, no SQL statement logging), a built Java jar, Node's compiled `dist/`, and for Python one Uvicorn worker per CPU if you compare multi-core throughput.
- Give each one a warm-up run first; the JVM in particular needs one before it reaches full speed.
- The database grows with every checkout run, and in Go every login stores a session in Redis for seven days. For repeatable numbers, start each implementation from a freshly migrated database and an empty Redis.

## Results

None are recorded here yet; paste the table `bench/report.py` prints, with the machine it ran on.

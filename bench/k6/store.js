// Load test for the store-service API. It only uses endpoints the Java, Python, Express and NestJS ports
// serve too, so the same script measures each of them.
//
//   k6 run -e BASE_URL=http://localhost:8080 -e SCENARIO=browse bench/k6/store.js
//
// SCENARIO is one of browse, auth or checkout; VUS and DURATION set the load. setup() creates its own
// category, store, products and buyers, so the database needs no fixtures beyond the seeded admin.
import http from "k6/http";
import { check, fail } from "k6";
import exec from "k6/execution";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const SCENARIO = __ENV.SCENARIO || "browse";
const VUS = Number(__ENV.VUS || 20);
const DURATION = __ENV.DURATION || "60s";
const ADMIN_EMAIL = __ENV.ADMIN_EMAIL || "admin@example.com";
const ADMIN_PASSWORD = __ENV.ADMIN_PASSWORD || "admin123";
const PRODUCTS = 20;
const PASSWORD = "bench-password";
// Unique names, so runs against the same database do not collide.
const RUN = `${Date.now().toString(36)}${Math.floor(Math.random() * 1e6).toString(36)}`;

if (!["browse", "auth", "checkout"].includes(SCENARIO)) {
  throw new Error(`unknown SCENARIO ${SCENARIO}: use browse, auth or checkout`);
}

export const options = {
  scenarios: {
    [SCENARIO]: { executor: "constant-vus", vus: VUS, duration: DURATION, exec: SCENARIO },
  },
  // Every request is expected to succeed; a failing one means the service is overloaded or rate limited.
  // The thresholds on the scenario's submetrics also put them in the summary without setup()'s requests.
  thresholds: {
    http_req_failed: ["rate<0.01"],
    checks: ["rate>0.99"],
    [`http_req_duration{scenario:${SCENARIO}}`]: ["max>=0"],
    [`http_reqs{scenario:${SCENARIO}}`]: ["count>=0"],
    [`http_req_failed{scenario:${SCENARIO}}`]: ["rate<0.01"],
  },
  summaryTrendStats: ["avg", "med", "p(95)", "p(99)", "max"],
  setupTimeout: "5m",
};

const JSON_HEADERS = { "Content-Type": "application/json" };

function post(path, body, token, extraHeaders = {}) {
  const headers = { ...JSON_HEADERS, ...extraHeaders };
  if (token) headers.Authorization = `Bearer ${token}`;
  return http.post(`${BASE_URL}${path}`, JSON.stringify(body), { headers, tags: { name: `POST ${path}` } });
}

function expect(res, status, what) {
  if (res.status !== status) {
    fail(`${what}: expected ${status}, got ${res.status} ${res.body}` +
      (res.status === 429 ? " (raise RATE_LIMIT_LOGIN, RATE_LIMIT_PUBLIC and RATE_LIMIT_AUTH for benchmarks)" : ""));
  }
  return res.json("data");
}

function register(email, name) {
  expect(post("/api/v1/auth/register", { email, password: PASSWORD, name }), 201, `register ${email}`);
}

function login(email, password = PASSWORD) {
  return expect(post("/api/v1/auth/login", { email, password }), 200, `login ${email}`).access_token;
}

export function setup() {
  const admin = login(ADMIN_EMAIL, ADMIN_PASSWORD);
  const category = expect(post("/api/v1/categories", { name: `Bench ${RUN}` }, admin), 201, "create category");

  const sellerEmail = `bench_seller_${RUN}@example.com`;
  register(sellerEmail, "Bench Seller");
  expect(post("/api/v1/stores", { name: `Bench Store ${RUN}`, description: "k6" }, login(sellerEmail)), 201, "create store");
  // A new login carries the seller role the store gave the user.
  const seller = login(sellerEmail);

  const productIds = [];
  for (let i = 0; i < PRODUCTS; i++) {
    const product = expect(post("/api/v1/products", {
      category_id: category.id,
      name: `Bench Product ${RUN} ${i}`,
      description: "k6",
      price: "10000",
      stock: 1000000000,
    }, seller), 201, "create product");
    productIds.push(product.id);
  }

  // One buyer per VU, so carts and their locks are never shared between VUs.
  const buyers = [];
  for (let i = 0; i < VUS; i++) {
    const email = `bench_buyer_${RUN}_${i}@example.com`;
    register(email, `Bench Buyer ${i}`);
    buyers.push({ email, token: login(email) });
  }
  return { productIds, buyers };
}

function buyerOf(data) {
  return data.buyers[(exec.vu.idInTest - 1) % data.buyers.length];
}

// Public reads: a product page and the product listing (product details are cached in Redis).
export function browse(data) {
  const id = data.productIds[exec.scenario.iterationInTest % data.productIds.length];
  const list = http.get(`${BASE_URL}/api/v1/products?page=1&per_page=20`, { tags: { name: "GET /api/v1/products" } });
  check(list, { "list products 200": (r) => r.status === 200 });
  const one = http.get(`${BASE_URL}/api/v1/products/${id}`, { tags: { name: "GET /api/v1/products/{id}" } });
  check(one, { "get product 200": (r) => r.status === 200 });
}

// Login, bound by bcrypt.
export function auth(data) {
  const res = post("/api/v1/auth/login", { email: buyerOf(data).email, password: PASSWORD });
  check(res, { "login 200": (r) => r.status === 200 });
}

// The write path: cart update under the Redis lock, then checkout in one transaction.
export function checkout(data) {
  const buyer = buyerOf(data);
  const productId = data.productIds[exec.scenario.iterationInTest % data.productIds.length];
  const add = post("/api/v1/cart/items", { product_id: productId, quantity: 1 }, buyer.token);
  if (!check(add, { "add to cart 200": (r) => r.status === 200 })) return;
  const order = post("/api/v1/orders", { shipping_address: "Jl. Benchmark 1" }, buyer.token, {
    "Idempotency-Key": `${RUN}-${exec.vu.idInTest}-${exec.vu.iterationInScenario}`,
  });
  check(order, { "checkout 201": (r) => r.status === 201 });
}

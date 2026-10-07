package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The integration tests run the repositories against a real PostgreSQL when
// TEST_DATABASE_URL is set, for example with the docker-compose database:
//
//	TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mini_go_ecommerce?sslmode=disable" \
//		go test ./store-service/internal/repository/
//
// They migrate a fresh schema and drop it afterwards, leaving the database's own
// data alone. Without the variable they are skipped.

var (
	integrationOnce   sync.Once
	integrationDB     *sqlx.DB
	integrationSchema string
	integrationErr    error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if integrationDB != nil {
		if _, err := integrationDB.Exec("DROP SCHEMA " + integrationSchema + " CASCADE"); err != nil {
			fmt.Fprintln(os.Stderr, "failed to drop test schema:", err)
		}
		integrationDB.Close()
	}
	os.Exit(code)
}

type testDatabase struct {
	db *sqlx.DB
}

func (d testDatabase) DB() *sqlx.DB { return d.db }
func (d testDatabase) Close() error { return nil }

func testDB(t *testing.T) testDatabase {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	integrationOnce.Do(func() {
		integrationDB, integrationSchema, integrationErr = openTestSchema(url)
	})
	require.NoError(t, integrationErr)
	return testDatabase{db: integrationDB}
}

// openTestSchema connects with search_path set to a new schema and applies the
// up migrations to it.
func openTestSchema(url string) (*sqlx.DB, string, error) {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, "", err
	}
	schema := fmt.Sprintf("repository_test_%d", time.Now().UnixNano())
	cfg.RuntimeParams["search_path"] = schema

	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	if _, err := db.Exec("CREATE SCHEMA " + schema); err != nil {
		db.Close()
		return nil, "", err
	}

	files, err := filepath.Glob("../../../migrations/*.up.sql")
	if err != nil {
		return db, schema, err
	}
	sort.Strings(files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return db, schema, err
		}
		if _, err := db.Exec(string(data)); err != nil {
			return db, schema, fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
	}
	return db, schema, nil
}

var errCacheMiss = errors.New("cache miss")

// memCache is an in-memory caches.Cache that stores JSON, like the Redis cache.
type memCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (c *memCache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.data[key]
	if !ok {
		return nil, errCacheMiss
	}
	return value, nil
}

func (c *memCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = data
	return nil
}

func (c *memCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *memCache) Exists(_ context.Context, key string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.data[key]
	return ok, nil
}

func (c *memCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string][]byte)
}

// fixtures holds the repositories under test and creates the rows tests need.
// Every row gets unique values, so the tests can share one schema.
type fixtures struct {
	ctx        context.Context
	db         *sqlx.DB
	cache      *memCache
	users      UserRepository
	stores     StoreRepository
	categories CategoryRepository
	products   ProductRepository
	carts      CartRepository
	orders     OrderRepository
	reviews    ReviewRepository
}

func newFixtures(t *testing.T) *fixtures {
	db := testDB(t)
	cache := &memCache{data: make(map[string][]byte)}
	return &fixtures{
		ctx:        context.Background(),
		db:         db.DB(),
		cache:      cache,
		users:      NewUserRepository(db),
		stores:     NewStoreRepository(db),
		categories: NewCategoryRepository(db),
		products:   NewProductRepository(db, cache),
		carts:      NewCartRepository(db, cache),
		orders:     NewOrderRepository(db, cache),
		reviews:    NewReviewRepository(db),
	}
}

func unique(prefix string) string {
	return prefix + "-" + uuid.NewString()[:8]
}

func (f *fixtures) user(t *testing.T) *model.User {
	t.Helper()
	user := &model.User{
		Email:    unique("user") + "@example.com",
		Password: "hash",
		Name:     unique("User"),
		Role:     constant.RoleBuyer,
	}
	require.NoError(t, f.users.Create(f.ctx, user))
	return user
}

func (f *fixtures) store(t *testing.T) *model.Store {
	t.Helper()
	store := &model.Store{UserID: f.user(t).ID, Name: unique("Store")}
	require.NoError(t, f.stores.Create(f.ctx, store))
	return store
}

func (f *fixtures) category(t *testing.T) *model.Category {
	t.Helper()
	category := &model.Category{Name: unique("Category")}
	require.NoError(t, f.categories.Create(f.ctx, category))
	return category
}

func (f *fixtures) product(t *testing.T, storeID, categoryID uuid.UUID, name, price string, stock int) *model.Product {
	t.Helper()
	product := &model.Product{
		StoreID:    storeID,
		CategoryID: categoryID,
		Name:       name,
		Price:      decimal.RequireFromString(price),
		Stock:      stock,
	}
	require.NoError(t, f.products.Create(f.ctx, product))
	return product
}

// stock reads a product's stock from the database, bypassing the cache.
func (f *fixtures) stock(t *testing.T, id uuid.UUID) int {
	t.Helper()
	var stock int
	require.NoError(t, f.db.GetContext(f.ctx, &stock, "SELECT stock FROM products WHERE id = $1", id))
	return stock
}

// placeOrder checks out quantity of product for buyer as one pending order with
// a pending payment. It does not fail the test itself, so goroutines can call it.
func (f *fixtures) placeOrder(buyer *model.User, product *model.Product, quantity int) (*model.Order, error) {
	orders, err := f.orders.CreateOrdersWithStock(f.ctx,
		[]StockReservation{{ProductID: product.ID, Quantity: quantity}},
		func(reserved []model.Product) ([]*model.Order, error) {
			total := reserved[0].Price.Mul(decimal.NewFromInt(int64(quantity)))
			return []*model.Order{{
				UserID:          buyer.ID,
				StoreID:         reserved[0].StoreID,
				Status:          constant.OrderStatusPending,
				TotalAmount:     total,
				ShippingAddress: "Jl. Test 1",
				OrderItems:      []model.OrderItem{{ProductID: reserved[0].ID, Quantity: quantity, Price: reserved[0].Price}},
				Payment:         &model.Payment{Method: model.PaymentMethodMock, Status: model.PaymentStatusPending, Amount: total},
			}}, nil
		})
	if err != nil {
		return nil, err
	}
	return orders[0], nil
}

func orderIDs(orders []model.Order) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	return ids
}

func productIDs(products []model.Product) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(products))
	for _, p := range products {
		ids = append(ids, p.ID)
	}
	return ids
}

func TestUserRepository_Integration(t *testing.T) {
	f := newFixtures(t)
	user := f.user(t)
	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.False(t, user.CreatedAt.IsZero())

	found, err := f.users.FindByEmail(f.ctx, strings.ToUpper(user.Email))
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)

	duplicate := &model.User{Email: user.Email, Password: "hash", Name: "Duplicate", Role: constant.RoleBuyer}
	assert.ErrorIs(t, f.users.Create(f.ctx, duplicate), ErrDuplicateKey)

	require.NoError(t, f.users.UpdateRole(f.ctx, user.ID, constant.RoleSeller))
	found, err = f.users.FindByID(f.ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, constant.RoleSeller, found.Role)

	_, err = f.users.FindByID(f.ctx, uuid.New())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestStoreRepository_Integration(t *testing.T) {
	f := newFixtures(t)
	store := f.store(t)

	second := &model.Store{UserID: store.UserID, Name: "Second store"}
	assert.ErrorIs(t, f.stores.Create(f.ctx, second), ErrDuplicateKey)

	store.Name, store.Description, store.LogoURL = "Renamed", "New description", "/uploads/stores/logo.png"
	require.NoError(t, f.stores.Update(f.ctx, store))
	found, err := f.stores.FindByUserID(f.ctx, store.UserID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", found.Name)
	assert.Equal(t, "New description", found.Description)
	assert.Equal(t, "/uploads/stores/logo.png", found.LogoURL)

	assert.ErrorIs(t, f.stores.Update(f.ctx, &model.Store{ID: uuid.New(), Name: "Missing"}), ErrNotFound)

	require.NoError(t, f.stores.Delete(f.ctx, store.ID))
	_, err = f.stores.FindByID(f.ctx, store.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCategoryRepository_Integration(t *testing.T) {
	f := newFixtures(t)
	first := &model.Category{Name: unique("A category")}
	second := &model.Category{Name: unique("B category")}
	require.NoError(t, f.categories.Create(f.ctx, first))
	require.NoError(t, f.categories.Create(f.ctx, second))

	assert.ErrorIs(t, f.categories.Create(f.ctx, &model.Category{Name: first.Name}), ErrDuplicateKey)
	renamed := *second
	renamed.Name = first.Name
	assert.ErrorIs(t, f.categories.Update(f.ctx, &renamed), ErrDuplicateKey)

	all, err := f.categories.FindAll(f.ctx)
	require.NoError(t, err)
	position := make(map[uuid.UUID]int, len(all))
	for i, c := range all {
		position[c.ID] = i
	}
	assert.Less(t, position[first.ID], position[second.ID], "categories are sorted by name")

	store := f.store(t)
	f.product(t, store.ID, first.ID, unique("Product"), "10.00", 1)
	assert.ErrorIs(t, f.categories.Delete(f.ctx, first.ID), ErrForeignKeyViolation)

	require.NoError(t, f.categories.Delete(f.ctx, second.ID))
	_, err = f.categories.FindByID(f.ctx, second.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestProductRepository_Integration(t *testing.T) {
	f := newFixtures(t)
	store := f.store(t)
	category := f.category(t)

	t.Run("create requires an existing category", func(t *testing.T) {
		product := &model.Product{StoreID: store.ID, CategoryID: uuid.New(), Name: "Orphan", Price: decimal.NewFromInt(1)}
		assert.ErrorIs(t, f.products.Create(f.ctx, product), ErrForeignKeyViolation)
	})

	t.Run("update never writes stock", func(t *testing.T) {
		product := f.product(t, store.ID, category.ID, unique("Mug"), "100.50", 5)
		require.NoError(t, f.products.UpdateStock(f.ctx, product.ID, 7))

		stale := *product // still holds stock 5
		stale.Name = "Renamed mug"
		stale.Price = decimal.RequireFromString("99.99")
		require.NoError(t, f.products.Update(f.ctx, &stale))
		assert.Equal(t, 7, stale.Stock, "Update reads back the stored stock")
		assert.Equal(t, 7, f.stock(t, product.ID))

		found, err := f.products.FindByID(f.ctx, product.ID)
		require.NoError(t, err)
		assert.Equal(t, "Renamed mug", found.Name)
		assert.True(t, decimal.RequireFromString("99.99").Equal(found.Price))
	})

	t.Run("find all filters, sorts and pages", func(t *testing.T) {
		shop := f.store(t)
		other := f.category(t)
		token := unique("findall")
		cheap := f.product(t, shop.ID, category.ID, token+" cheap", "5.00", 1)
		mid := f.product(t, shop.ID, category.ID, token+" mid", "50.00", 1)
		pricey := f.product(t, shop.ID, other.ID, token+" pricey", "500.00", 1)

		products, total, err := f.products.FindAll(f.ctx, model.ProductFilter{
			StoreID: shop.ID.String(), Search: strings.ToUpper(token), MinPrice: "10",
			SortBy: "price", SortOrder: "asc", Page: 1, PerPage: 10,
		})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Equal(t, []uuid.UUID{mid.ID, pricey.ID}, productIDs(products))

		products, total, err = f.products.FindAll(f.ctx, model.ProductFilter{
			StoreID: shop.ID.String(), CategoryID: other.ID.String(), Page: 1, PerPage: 10,
		})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, []uuid.UUID{pricey.ID}, productIDs(products))

		products, total, err = f.products.FindAll(f.ctx, model.ProductFilter{
			StoreID: shop.ID.String(), SortBy: "price", SortOrder: "desc", Page: 2, PerPage: 2,
		})
		require.NoError(t, err)
		assert.EqualValues(t, 3, total, "the total ignores pagination")
		assert.Equal(t, []uuid.UUID{cheap.ID}, productIDs(products))
	})

	t.Run("delete refuses products that orders reference", func(t *testing.T) {
		ordered := f.product(t, store.ID, category.ID, unique("Lamp"), "20.00", 3)
		_, err := f.placeOrder(f.user(t), ordered, 1)
		require.NoError(t, err)
		assert.ErrorIs(t, f.products.Delete(f.ctx, ordered.ID), ErrForeignKeyViolation)

		unused := f.product(t, store.ID, category.ID, unique("Vase"), "20.00", 3)
		require.NoError(t, f.products.Delete(f.ctx, unused.ID))
		_, err = f.products.FindByID(f.ctx, unused.ID)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestCartRepository_Integration(t *testing.T) {
	f := newFixtures(t)
	buyer := f.user(t)
	store := f.store(t)
	category := f.category(t)
	first := f.product(t, store.ID, category.ID, unique("Pen"), "10.00", 5)
	second := f.product(t, store.ID, category.ID, unique("Book"), "20.00", 5)

	cart := &model.Cart{UserID: buyer.ID, Items: []model.CartItem{
		{ProductID: first.ID, Quantity: 2},
		{ProductID: second.ID, Quantity: 1},
	}}
	require.NoError(t, f.carts.SaveCart(f.ctx, cart))

	// Without the cached copy, GetCart reads the PostgreSQL backup.
	f.cache.clear()
	loaded, err := f.carts.GetCart(f.ctx, buyer.ID)
	require.NoError(t, err)
	require.Len(t, loaded.Items, 2)
	items := make(map[uuid.UUID]model.CartItem)
	for _, item := range loaded.Items {
		items[item.ProductID] = item
	}
	assert.Equal(t, 2, items[first.ID].Quantity)
	assert.Equal(t, first.Name, items[first.ID].Name)
	assert.True(t, first.Price.Equal(items[first.ID].Price))
	assert.Equal(t, 1, items[second.ID].Quantity)

	cart.Items = cart.Items[:1]
	require.NoError(t, f.carts.SaveCart(f.ctx, cart))
	f.cache.clear()
	loaded, err = f.carts.GetCart(f.ctx, buyer.ID)
	require.NoError(t, err)
	assert.Len(t, loaded.Items, 1, "saving replaces the stored items")

	require.NoError(t, f.carts.DeleteCart(f.ctx, buyer.ID))
	loaded, err = f.carts.GetCart(f.ctx, buyer.ID)
	require.NoError(t, err)
	assert.NotNil(t, loaded.Items, "an empty cart has an empty list, not null")
	assert.Empty(t, loaded.Items)
}

func TestOrderRepository_Integration(t *testing.T) {
	f := newFixtures(t)
	buyer := f.user(t)
	store := f.store(t)
	category := f.category(t)

	t.Run("checkout reserves stock and stores the order", func(t *testing.T) {
		product := f.product(t, store.ID, category.ID, unique("Mug"), "25.00", 5)
		order, err := f.placeOrder(buyer, product, 2)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, order.ID)
		require.Len(t, order.OrderItems, 1)
		assert.NotEqual(t, uuid.Nil, order.OrderItems[0].ID)
		require.NotNil(t, order.Payment)
		assert.NotEqual(t, uuid.Nil, order.Payment.ID)
		assert.Equal(t, 3, f.stock(t, product.ID))

		found, err := f.orders.FindByID(f.ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, constant.OrderStatusPending, found.Status)
		assert.True(t, decimal.RequireFromString("50.00").Equal(found.TotalAmount))
		require.Len(t, found.OrderItems, 1)
		assert.Equal(t, 2, found.OrderItems[0].Quantity)
		require.NotNil(t, found.Payment)
		assert.Equal(t, model.PaymentStatusPending, found.Payment.Status)
		assert.Nil(t, found.Payment.PaidAt)
	})

	t.Run("a failed reservation writes nothing", func(t *testing.T) {
		scarce := f.product(t, store.ID, category.ID, unique("Scarce"), "10.00", 1)
		plenty := f.product(t, store.ID, category.ID, unique("Plenty"), "10.00", 5)
		build := func([]model.Product) ([]*model.Order, error) {
			t.Error("build must not run when a reservation fails")
			return nil, nil
		}

		_, err := f.orders.CreateOrdersWithStock(f.ctx, []StockReservation{
			{ProductID: plenty.ID, Quantity: 1},
			{ProductID: scarce.ID, Quantity: 2},
		}, build)
		var insufficient *ErrInsufficientStock
		require.ErrorAs(t, err, &insufficient)
		assert.Equal(t, scarce.Name, insufficient.ProductName)
		assert.Equal(t, 5, f.stock(t, plenty.ID), "the earlier reservation is rolled back")
		assert.Equal(t, 1, f.stock(t, scarce.ID))

		_, err = f.orders.CreateOrdersWithStock(f.ctx, []StockReservation{{ProductID: uuid.New(), Quantity: 1}}, build)
		var notFound *ErrProductNotFound
		assert.ErrorAs(t, err, &notFound)
	})

	t.Run("concurrent checkouts cannot oversell", func(t *testing.T) {
		product := f.product(t, store.ID, category.ID, unique("Last one"), "10.00", 1)
		buyers := []*model.User{f.user(t), f.user(t), f.user(t)}

		errs := make([]error, len(buyers))
		var wg sync.WaitGroup
		for i, b := range buyers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = f.placeOrder(b, product, 1)
			}()
		}
		wg.Wait()

		succeeded := 0
		for _, err := range errs {
			if err == nil {
				succeeded++
				continue
			}
			var insufficient *ErrInsufficientStock
			assert.ErrorAs(t, err, &insufficient)
		}
		assert.Equal(t, 1, succeeded)
		assert.Equal(t, 0, f.stock(t, product.ID))
	})

	t.Run("lists by buyer and by store, newest first, with details", func(t *testing.T) {
		shopper := f.user(t)
		shop := f.store(t)
		product := f.product(t, shop.ID, category.ID, unique("Lamp"), "15.00", 10)
		older, err := f.placeOrder(shopper, product, 1)
		require.NoError(t, err)
		newer, err := f.placeOrder(shopper, product, 2)
		require.NoError(t, err)

		orders, total, err := f.orders.FindByUserID(f.ctx, shopper.ID, 1, 10)
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Equal(t, []uuid.UUID{newer.ID, older.ID}, orderIDs(orders))
		for _, o := range orders {
			assert.Len(t, o.OrderItems, 1)
			assert.NotNil(t, o.Payment)
		}

		page, total, err := f.orders.FindByStoreID(f.ctx, shop.ID, 2, 1)
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Equal(t, []uuid.UUID{older.ID}, orderIDs(page))
	})

	t.Run("status updates are compare-and-set", func(t *testing.T) {
		product := f.product(t, store.ID, category.ID, unique("Pencil"), "3.00", 10)
		order, err := f.placeOrder(buyer, product, 1)
		require.NoError(t, err)

		ok, err := f.orders.UpdateStatusIfCurrent(f.ctx, order.ID, constant.OrderStatusPaid, constant.OrderStatusProcessing)
		require.NoError(t, err)
		assert.False(t, ok, "the order is still pending")

		ok, err = f.orders.MarkPaymentSucceeded(f.ctx, order.ID)
		require.NoError(t, err)
		assert.True(t, ok)
		ok, err = f.orders.MarkPaymentSucceeded(f.ctx, order.ID)
		require.NoError(t, err)
		assert.False(t, ok, "a duplicate payment result changes nothing")

		found, err := f.orders.FindByID(f.ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, constant.OrderStatusPaid, found.Status)
		require.NotNil(t, found.Payment)
		assert.Equal(t, model.PaymentStatusSuccess, found.Payment.Status)
		assert.NotNil(t, found.Payment.PaidAt)

		ok, err = f.orders.UpdateStatusIfCurrent(f.ctx, order.ID, constant.OrderStatusPaid, constant.OrderStatusProcessing)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("cancellation and failed payments restock once", func(t *testing.T) {
		product := f.product(t, store.ID, category.ID, unique("Plate"), "8.00", 10)
		cancelled, err := f.placeOrder(buyer, product, 3)
		require.NoError(t, err)
		failed, err := f.placeOrder(buyer, product, 2)
		require.NoError(t, err)
		assert.Equal(t, 5, f.stock(t, product.ID))

		ok, err := f.orders.CancelAndRestock(f.ctx, cancelled.ID, constant.OrderStatusPending)
		require.NoError(t, err)
		assert.True(t, ok)
		ok, err = f.orders.CancelAndRestock(f.ctx, cancelled.ID, constant.OrderStatusPending)
		require.NoError(t, err)
		assert.False(t, ok, "cancelling again changes nothing")
		assert.Equal(t, 8, f.stock(t, product.ID))

		ok, err = f.orders.MarkPaymentFailed(f.ctx, failed.ID)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, 10, f.stock(t, product.ID))

		wantPayment := map[uuid.UUID]string{
			cancelled.ID: model.PaymentStatusCancelled,
			failed.ID:    model.PaymentStatusFailed,
		}
		for id, paymentStatus := range wantPayment {
			found, err := f.orders.FindByID(f.ctx, id)
			require.NoError(t, err)
			assert.Equal(t, constant.OrderStatusCancelled, found.Status)
			require.NotNil(t, found.Payment)
			assert.Equal(t, paymentStatus, found.Payment.Status)
		}
	})

	t.Run("stale pending orders", func(t *testing.T) {
		product := f.product(t, store.ID, category.ID, unique("Cup"), "1.00", 10)
		order, err := f.placeOrder(buyer, product, 1)
		require.NoError(t, err)

		stale, err := f.orders.FindStalePending(f.ctx, time.Now().Add(time.Minute), 1000)
		require.NoError(t, err)
		assert.Contains(t, orderIDs(stale), order.ID)

		stale, err = f.orders.FindStalePending(f.ctx, time.Now().Add(-time.Hour), 1000)
		require.NoError(t, err)
		assert.NotContains(t, orderIDs(stale), order.ID)
	})
}

func TestReviewRepository_Integration(t *testing.T) {
	f := newFixtures(t)
	buyer := f.user(t)
	store := f.store(t)
	product := f.product(t, store.ID, f.category(t).ID, unique("Chair"), "12.00", 5)

	purchased, err := f.reviews.HasUserPurchased(f.ctx, buyer.ID, product.ID)
	require.NoError(t, err)
	assert.False(t, purchased)

	order, err := f.placeOrder(buyer, product, 1)
	require.NoError(t, err)
	purchased, err = f.reviews.HasUserPurchased(f.ctx, buyer.ID, product.ID)
	require.NoError(t, err)
	assert.False(t, purchased, "a pending order does not count as a purchase")

	ok, err := f.orders.UpdateStatusIfCurrent(f.ctx, order.ID, constant.OrderStatusPending, constant.OrderStatusShipped)
	require.NoError(t, err)
	require.True(t, ok)
	purchased, err = f.reviews.HasUserPurchased(f.ctx, buyer.ID, product.ID)
	require.NoError(t, err)
	assert.True(t, purchased)

	review := &model.Review{UserID: buyer.ID, ProductID: product.ID, Rating: 4, Comment: "Solid"}
	require.NoError(t, f.reviews.Create(f.ctx, review))
	assert.NotEqual(t, uuid.Nil, review.ID)

	duplicate := &model.Review{UserID: buyer.ID, ProductID: product.ID, Rating: 1}
	assert.ErrorIs(t, f.reviews.Create(f.ctx, duplicate), ErrDuplicateKey)

	reviewed, err := f.reviews.HasUserReviewed(f.ctx, buyer.ID, product.ID)
	require.NoError(t, err)
	assert.True(t, reviewed)

	reviews, total, err := f.reviews.FindByProductID(f.ctx, product.ID, 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, reviews, 1)
	assert.Equal(t, buyer.Name, reviews[0].UserName)
	assert.Equal(t, "Solid", reviews[0].Comment)
}

package repository

import (
	"context"
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
	rediscache "github.com/1tsndre/mini-go-project/store-service/internal/repository/caches/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Skipped unless TEST_DATABASE_URL is set. Everything runs in a temporary schema.

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

func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	integrationOnce.Do(func() {
		integrationDB, integrationSchema, integrationErr = openTestSchema(url)
	})
	require.NoError(t, integrationErr)
	return integrationDB
}

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

type fixtures struct {
	ctx        context.Context
	db         *sqlx.DB
	cache      *miniredis.Miniredis
	users      UserRepository
	stores     StoreRepository
	categories CategoryRepository
	products   ProductRepository
	carts      CartRepository
	orders     OrderRepository
	reviews    ReviewRepository
}

func newFixtures(t *testing.T) *fixtures {
	var (
		db         = testDB(t)
		cache      = miniredis.RunT(t)
		redisCache = rediscache.NewRedisCache(redis.NewClient(&redis.Options{Addr: cache.Addr()}))
	)

	return &fixtures{
		ctx:        context.Background(),
		db:         db,
		cache:      cache,
		users:      NewUserRepository(db),
		stores:     NewStoreRepository(db),
		categories: NewCategoryRepository(db),
		products:   NewProductRepository(db, redisCache),
		carts:      NewCartRepository(db, redisCache),
		orders:     NewOrderRepository(db, redisCache),
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

func (f *fixtures) stock(t *testing.T, id uuid.UUID) int {
	t.Helper()
	var stock int

	require.NoError(t, f.db.GetContext(f.ctx, &stock, "SELECT stock FROM products WHERE id = $1", id))
	return stock
}

// Orders the buyer's one-item cart. Returns the error, so goroutines can call it.
func (f *fixtures) checkout(buyer *model.User) (*model.Order, error) {
	orders, err := f.orders.CreateOrdersFromCart(f.ctx, buyer.ID,
		func(items []model.CartItem, reserved []model.Product) []*model.Order {
			total := reserved[0].Price.Mul(decimal.NewFromInt(int64(items[0].Quantity)))
			return []*model.Order{{
				UserID:          buyer.ID,
				StoreID:         reserved[0].StoreID,
				Status:          constant.OrderStatusPending,
				TotalAmount:     total,
				ShippingAddress: "Jl. Test 1",
				OrderItems:      []model.OrderItem{{ProductID: reserved[0].ID, Quantity: items[0].Quantity, Price: reserved[0].Price}},
				Payment:         &model.Payment{Method: constant.PaymentMethodMock, Status: constant.PaymentStatusPending, Amount: total},
			}}
		})
	if err != nil {
		return nil, err
	}
	return orders[0], nil
}

// Returns the error, so goroutines can call it.
func (f *fixtures) placeOrder(buyer *model.User, product *model.Product, quantity int) (*model.Order, error) {
	cart := &model.Cart{UserID: buyer.ID, Items: []model.CartItem{{ProductID: product.ID, Quantity: quantity}}}
	if err := f.carts.Save(f.ctx, cart); err != nil {
		return nil, err
	}
	return f.checkout(buyer)
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
	var (
		f    = newFixtures(t)
		user = f.user(t)
	)

	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.False(t, user.CreatedAt.IsZero())

	found, err := f.users.FindByEmail(f.ctx, user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)

	duplicate := &model.User{Email: user.Email, Password: "hash", Name: "Duplicate", Role: constant.RoleBuyer}
	assert.ErrorIs(t, f.users.Create(f.ctx, duplicate), ErrDuplicateKey)

	found, err = f.users.FindByID(f.ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, constant.RoleBuyer, found.Role)

	_, err = f.users.FindByID(f.ctx, uuid.New())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestStoreRepository_Integration(t *testing.T) {
	var (
		f     = newFixtures(t)
		store = f.store(t)
	)

	owner, err := f.users.FindByID(f.ctx, store.UserID)
	require.NoError(t, err)
	assert.Equal(t, constant.RoleSeller, owner.Role, "opening a store makes its owner a seller")

	second := &model.Store{UserID: store.UserID, Name: "Second store"}
	assert.ErrorIs(t, f.stores.Create(f.ctx, second), ErrDuplicateKey)

	stale := *store
	store.LogoURL = "/uploads/stores/logo.png"
	require.NoError(t, f.stores.UpdateLogo(f.ctx, store))

	stale.Name, stale.Description = "Renamed", "New description"
	require.NoError(t, f.stores.Update(f.ctx, &stale))
	found, err := f.stores.FindByUserID(f.ctx, store.UserID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", found.Name)
	assert.Equal(t, "New description", found.Description)
	assert.Equal(t, "/uploads/stores/logo.png", found.LogoURL, "a stale Update does not undo the logo")

	assert.ErrorIs(t, f.stores.Update(f.ctx, &model.Store{ID: uuid.New(), Name: "Missing"}), ErrNotFound)
	assert.ErrorIs(t, f.stores.UpdateLogo(f.ctx, &model.Store{ID: uuid.New()}), ErrNotFound)

	_, err = f.stores.FindByID(f.ctx, uuid.New())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCategoryRepository_Integration(t *testing.T) {
	var (
		f      = newFixtures(t)
		first  = &model.Category{Name: unique("A category")}
		second = &model.Category{Name: unique("B category")}
	)

	require.NoError(t, f.categories.Create(f.ctx, first))
	require.NoError(t, f.categories.Create(f.ctx, second))

	assert.ErrorIs(t, f.categories.Create(f.ctx, &model.Category{Name: first.Name}), ErrDuplicateKey)
	renamed := *second
	renamed.Name = first.Name
	assert.ErrorIs(t, f.categories.Update(f.ctx, &renamed), ErrDuplicateKey)

	second.Name = unique("C category")
	require.NoError(t, f.categories.Update(f.ctx, second))

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
	assert.ErrorIs(t, f.categories.Delete(f.ctx, second.ID), ErrNotFound)
	assert.ErrorIs(t, f.categories.Update(f.ctx, second), ErrNotFound)
}

func TestProductRepository_Integration(t *testing.T) {
	var (
		f        = newFixtures(t)
		store    = f.store(t)
		category = f.category(t)
	)

	t.Run("create requires an existing category", func(t *testing.T) {
		product := &model.Product{StoreID: store.ID, CategoryID: uuid.New(), Name: "Orphan", Price: decimal.NewFromInt(1)}
		assert.ErrorIs(t, f.products.Create(f.ctx, product), ErrForeignKeyViolation)
	})

	t.Run("update never writes stock or image", func(t *testing.T) {
		product := f.product(t, store.ID, category.ID, unique("Mug"), "100.50", 5)
		require.NoError(t, f.products.UpdateStock(f.ctx, product.ID, 7))

		withImage := *product
		withImage.ImageURL = "products/mug.png"
		require.NoError(t, f.products.UpdateImage(f.ctx, &withImage))

		stale := *product // still holds stock 5 and no image
		stale.Name = "Renamed mug"
		stale.Price = decimal.RequireFromString("99.99")
		require.NoError(t, f.products.Update(f.ctx, &stale))
		assert.Equal(t, 7, stale.Stock, "Update reads back the stored stock")
		assert.Equal(t, 7, f.stock(t, product.ID))
		assert.Equal(t, "products/mug.png", stale.ImageURL, "Update reads back the stored image")

		found, err := f.products.FindByID(f.ctx, product.ID)
		require.NoError(t, err)
		assert.Equal(t, "Renamed mug", found.Name)
		assert.True(t, decimal.RequireFromString("99.99").Equal(found.Price))
	})

	t.Run("find all filters, sorts and pages", func(t *testing.T) {
		var (
			shop   = f.store(t)
			other  = f.category(t)
			token  = unique("findall")
			cheap  = f.product(t, shop.ID, category.ID, token+" cheap", "5.00", 1)
			mid    = f.product(t, shop.ID, category.ID, token+" mid", "50.00", 1)
			pricey = f.product(t, shop.ID, other.ID, token+" pricey", "500.00", 1)
		)

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

		products, _, err = f.products.FindAll(f.ctx, model.ProductFilter{Search: unique("missing"), Page: 1, PerPage: 10})
		require.NoError(t, err)
		assert.NotNil(t, products, "no products is an empty list, not null")
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
	var (
		f        = newFixtures(t)
		buyer    = f.user(t)
		store    = f.store(t)
		category = f.category(t)
		first    = f.product(t, store.ID, category.ID, unique("Pen"), "10.00", 5)
		second   = f.product(t, store.ID, category.ID, unique("Book"), "20.00", 5)
		cart     = &model.Cart{UserID: buyer.ID, Items: []model.CartItem{
			{ProductID: first.ID, Quantity: 2},
			{ProductID: second.ID, Quantity: 1},
		}}
		items map[uuid.UUID]model.CartItem
	)

	require.NoError(t, f.carts.Save(f.ctx, cart))

	// Without the cached copy, FindByUserID reads PostgreSQL.
	f.cache.FlushAll()
	loaded, err := f.carts.FindByUserID(f.ctx, buyer.ID)
	require.NoError(t, err)
	require.Len(t, loaded.Items, 2)
	items = make(map[uuid.UUID]model.CartItem)
	for _, item := range loaded.Items {
		items[item.ProductID] = item
	}
	assert.Equal(t, 2, items[first.ID].Quantity)
	assert.Equal(t, first.Name, items[first.ID].Name)
	assert.True(t, first.Price.Equal(items[first.ID].Price))
	assert.Equal(t, 1, items[second.ID].Quantity)

	cart.Items = cart.Items[:1]
	require.NoError(t, f.carts.Save(f.ctx, cart))
	f.cache.FlushAll()
	loaded, err = f.carts.FindByUserID(f.ctx, buyer.ID)
	require.NoError(t, err)
	assert.Len(t, loaded.Items, 1, "saving replaces the stored items")

	require.NoError(t, f.carts.Save(f.ctx, &model.Cart{UserID: buyer.ID}))
	f.cache.FlushAll()
	loaded, err = f.carts.FindByUserID(f.ctx, buyer.ID)
	require.NoError(t, err)
	assert.NotNil(t, loaded.Items, "an empty cart has an empty list, not null")
	assert.Empty(t, loaded.Items)
}

func TestOrderRepository_Integration(t *testing.T) {
	var (
		f        = newFixtures(t)
		buyer    = f.user(t)
		store    = f.store(t)
		category = f.category(t)
	)

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
		assert.Equal(t, constant.PaymentStatusPending, found.Payment.Status)
		assert.Nil(t, found.Payment.PaidAt)
	})

	t.Run("a failed reservation writes nothing and keeps the cart", func(t *testing.T) {
		var (
			shopper = f.user(t)
			scarce  = f.product(t, store.ID, category.ID, unique("Scarce"), "10.00", 1)
			plenty  = f.product(t, store.ID, category.ID, unique("Plenty"), "10.00", 5)
		)

		build := func([]model.CartItem, []model.Product) []*model.Order {
			t.Error("build must not run when a reservation fails")
			return nil
		}

		require.NoError(t, f.carts.Save(f.ctx, &model.Cart{UserID: shopper.ID, Items: []model.CartItem{
			{ProductID: plenty.ID, Quantity: 1},
			{ProductID: scarce.ID, Quantity: 2},
		}}))
		_, err := f.orders.CreateOrdersFromCart(f.ctx, shopper.ID, build)
		require.ErrorIs(t, err, ErrInsufficientStock)
		assert.Contains(t, err.Error(), scarce.Name)
		assert.Equal(t, 5, f.stock(t, plenty.ID), "the earlier reservation is rolled back")
		assert.Equal(t, 1, f.stock(t, scarce.ID))
		cart, err := f.carts.FindByUserID(f.ctx, shopper.ID)
		require.NoError(t, err)
		assert.Len(t, cart.Items, 2)

		_, err = f.orders.CreateOrdersFromCart(f.ctx, f.user(t).ID, build)
		assert.ErrorIs(t, err, ErrCartEmpty)
	})

	t.Run("a cart is ordered once, however many checkouts run at once", func(t *testing.T) {
		var (
			shopper = f.user(t)
			product = f.product(t, store.ID, category.ID, unique("Kettle"), "30.00", 10)
			errs    = make([]error, 3)
			wg      sync.WaitGroup
		)

		require.NoError(t, f.carts.Save(f.ctx, &model.Cart{UserID: shopper.ID, Items: []model.CartItem{{ProductID: product.ID, Quantity: 2}}}))

		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = f.checkout(shopper)
			}()
		}
		wg.Wait()

		succeeded := 0
		for _, err := range errs {
			if err == nil {
				succeeded++
				continue
			}
			assert.ErrorIs(t, err, ErrCartEmpty)
		}
		assert.Equal(t, 1, succeeded)
		assert.Equal(t, 8, f.stock(t, product.ID))
		cart, err := f.carts.FindByUserID(f.ctx, shopper.ID)
		require.NoError(t, err)
		assert.Empty(t, cart.Items, "the cached cart is gone too")
	})

	t.Run("concurrent checkouts cannot oversell", func(t *testing.T) {
		var (
			product = f.product(t, store.ID, category.ID, unique("Last one"), "10.00", 1)
			buyers  = []*model.User{f.user(t), f.user(t), f.user(t)}
			errs    = make([]error, len(buyers))
			wg      sync.WaitGroup
		)

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
			assert.ErrorIs(t, err, ErrInsufficientStock)
		}
		assert.Equal(t, 1, succeeded)
		assert.Equal(t, 0, f.stock(t, product.ID))
	})

	t.Run("lists by buyer and by store, newest first, with details", func(t *testing.T) {
		var (
			shopper = f.user(t)
			shop    = f.store(t)
			product = f.product(t, shop.ID, category.ID, unique("Lamp"), "15.00", 10)
		)

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
		assert.Equal(t, constant.PaymentStatusSuccess, found.Payment.Status)
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
			cancelled.ID: constant.PaymentStatusCancelled,
			failed.ID:    constant.PaymentStatusFailed,
		}
		for id, paymentStatus := range wantPayment {
			found, err := f.orders.FindByID(f.ctx, id)
			require.NoError(t, err)
			assert.Equal(t, constant.OrderStatusCancelled, found.Status)
			require.NotNil(t, found.Payment)
			assert.Equal(t, paymentStatus, found.Payment.Status)
		}
	})

	t.Run("cancelling a paid order refunds it", func(t *testing.T) {
		product := f.product(t, store.ID, category.ID, unique("Bowl"), "8.00", 10)
		order, err := f.placeOrder(buyer, product, 3)
		require.NoError(t, err)
		_, err = f.orders.MarkPaymentSucceeded(f.ctx, order.ID)
		require.NoError(t, err)

		ok, err := f.orders.CancelAndRestock(f.ctx, order.ID, constant.OrderStatusPaid)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, 10, f.stock(t, product.ID))

		found, err := f.orders.FindByID(f.ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, constant.PaymentStatusRefunded, found.Payment.Status)
	})

	t.Run("pending orders", func(t *testing.T) {
		product := f.product(t, store.ID, category.ID, unique("Cup"), "1.00", 10)
		waiting, err := f.placeOrder(buyer, product, 1)
		require.NoError(t, err)
		paid, err := f.placeOrder(buyer, product, 1)
		require.NoError(t, err)
		_, err = f.orders.MarkPaymentSucceeded(f.ctx, paid.ID)
		require.NoError(t, err)

		pending, err := f.orders.FindPending(f.ctx)
		require.NoError(t, err)
		ids := make([]uuid.UUID, 0, len(pending))
		for _, o := range pending {
			ids = append(ids, o.ID)
		}
		assert.Contains(t, ids, waiting.ID)
		assert.NotContains(t, ids, paid.ID)
	})
}

func TestReviewRepository_Integration(t *testing.T) {
	var (
		f       = newFixtures(t)
		buyer   = f.user(t)
		store   = f.store(t)
		product = f.product(t, store.ID, f.category(t).ID, unique("Chair"), "12.00", 5)
	)

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
	assert.Equal(t, buyer.Name, review.UserName)

	duplicate := &model.Review{UserID: buyer.ID, ProductID: product.ID, Rating: 1}
	assert.ErrorIs(t, f.reviews.Create(f.ctx, duplicate), ErrDuplicateKey)

	reviews, total, err := f.reviews.FindByProductID(f.ctx, product.ID, 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, reviews, 1)
	assert.Equal(t, buyer.Name, reviews[0].UserName)
	assert.Equal(t, "Solid", reviews[0].Comment)
}

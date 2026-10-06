package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/go-redsync/redsync/v4"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CartService interface {
	GetCart(ctx context.Context, userID uuid.UUID) (*model.CartResponse, error)
	AddItem(ctx context.Context, userID uuid.UUID, req model.AddCartItemRequest) (*model.CartResponse, error)
	UpdateItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.UpdateCartItemRequest) (*model.CartResponse, error)
	RemoveItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID) (*model.CartResponse, error)
}

type cartService struct {
	cartRepo    repository.CartRepository
	productRepo repository.ProductRepository
	redsync     *redsync.Redsync
}

func NewCartService(cartRepo repository.CartRepository, productRepo repository.ProductRepository, rs *redsync.Redsync) CartService {
	return &cartService{
		cartRepo:    cartRepo,
		productRepo: productRepo,
		redsync:     rs,
	}
}

// lockUserCart takes the per-user cart lock shared by cart updates and checkout,
// so they never interleave on the same cart. A nil rs (unit tests) skips locking.
func lockUserCart(rs *redsync.Redsync, userID uuid.UUID) (func(), error) {
	if rs == nil {
		return func() {}, nil
	}
	mutex := rs.NewMutex(fmt.Sprintf(constant.KeyCartLock, userID.String()))
	if err := mutex.Lock(); err != nil {
		return nil, errors.New("failed to acquire cart lock, please try again")
	}
	return func() { mutex.Unlock() }, nil
}

func (s *cartService) GetCart(ctx context.Context, userID uuid.UUID) (*model.CartResponse, error) {
	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to fetch cart")
	}
	s.refreshItems(ctx, cart)
	return s.toCartResponse(cart), nil
}

// refreshItems sets each item's name, price and image to the product's current
// values. The stored values are a snapshot from when the item was added, while
// checkout charges the current price, so showing the snapshot could show a total
// the buyer will not actually pay. Products the caller already loaded are reused;
// if a product cannot be loaded, its stored values are kept.
func (s *cartService) refreshItems(ctx context.Context, cart *model.Cart, loaded ...*model.Product) {
	known := make(map[uuid.UUID]*model.Product, len(loaded))
	for _, p := range loaded {
		known[p.ID] = p
	}
	for i := range cart.Items {
		item := &cart.Items[i]
		product, ok := known[item.ProductID]
		if !ok {
			var err error
			if product, err = s.productRepo.FindByID(ctx, item.ProductID); err != nil {
				continue
			}
		}
		item.Name = product.Name
		item.Price = product.Price
		item.ImageURL = product.ImageURL
	}
}

func (s *cartService) AddItem(ctx context.Context, userID uuid.UUID, req model.AddCartItemRequest) (*model.CartResponse, error) {
	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		return nil, errors.New("invalid product_id")
	}

	if req.Quantity <= 0 {
		return nil, errors.New("quantity must be greater than 0")
	}

	product, err := s.productRepo.FindByID(ctx, productID)
	if err != nil {
		return nil, errors.New("product not found")
	}

	if product.Stock < req.Quantity {
		return nil, ErrInsufficientStock
	}

	unlock, err := lockUserCart(s.redsync, userID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	// GetCart returns an empty cart when the user has none, so an error here is a
	// real failure. Saving over it would replace the whole cart with this one item.
	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to load cart")
	}

	found := false
	for i, item := range cart.Items {
		if item.ProductID == productID {
			if product.Stock < item.Quantity+req.Quantity {
				return nil, ErrInsufficientStock
			}
			cart.Items[i].Quantity += req.Quantity
			found = true
			break
		}
	}

	if !found {
		cart.Items = append(cart.Items, model.CartItem{
			ProductID: productID,
			Name:      product.Name,
			Price:     product.Price,
			Quantity:  req.Quantity,
			ImageURL:  product.ImageURL,
		})
	}

	s.refreshItems(ctx, cart, product)
	cart.UpdatedAt = time.Now()
	if err := s.cartRepo.SaveCart(ctx, cart); err != nil {
		return nil, errors.New("failed to save cart")
	}

	return s.toCartResponse(cart), nil
}

func (s *cartService) UpdateItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.UpdateCartItemRequest) (*model.CartResponse, error) {
	if req.Quantity <= 0 {
		return nil, errors.New("quantity must be greater than 0")
	}

	unlock, err := lockUserCart(s.redsync, userID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to load cart")
	}

	idx := -1
	for i, item := range cart.Items {
		if item.ProductID == productID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return nil, errors.New("item not found in cart")
	}

	product, err := s.productRepo.FindByID(ctx, productID)
	if err != nil {
		return nil, errors.New("product not found")
	}
	if product.Stock < req.Quantity {
		return nil, ErrInsufficientStock
	}
	cart.Items[idx].Quantity = req.Quantity

	s.refreshItems(ctx, cart, product)
	cart.UpdatedAt = time.Now()
	if err := s.cartRepo.SaveCart(ctx, cart); err != nil {
		return nil, errors.New("failed to save cart")
	}

	return s.toCartResponse(cart), nil
}

func (s *cartService) RemoveItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID) (*model.CartResponse, error) {
	unlock, err := lockUserCart(s.redsync, userID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to load cart")
	}

	found := false
	for i, item := range cart.Items {
		if item.ProductID == productID {
			cart.Items = append(cart.Items[:i], cart.Items[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return nil, errors.New("item not found in cart")
	}

	s.refreshItems(ctx, cart)
	cart.UpdatedAt = time.Now()
	if err := s.cartRepo.SaveCart(ctx, cart); err != nil {
		return nil, errors.New("failed to save cart")
	}

	return s.toCartResponse(cart), nil
}

func (s *cartService) toCartResponse(cart *model.Cart) *model.CartResponse {
	total := decimal.NewFromInt(0)
	items := make([]model.CartItemResponse, 0, len(cart.Items))

	for _, item := range cart.Items {
		subtotal := item.Price.Mul(decimal.NewFromInt(int64(item.Quantity)))
		total = total.Add(subtotal)
		items = append(items, model.CartItemResponse{
			ProductID: item.ProductID,
			Name:      item.Name,
			Price:     item.Price,
			Quantity:  item.Quantity,
			Subtotal:  subtotal,
			ImageURL:  item.ImageURL,
		})
	}

	return &model.CartResponse{
		Items:     items,
		Total:     total,
		UpdatedAt: cart.UpdatedAt,
	}
}

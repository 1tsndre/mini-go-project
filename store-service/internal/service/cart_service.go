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

func (s *cartService) lockCart(userID uuid.UUID) (func(), error) {
	if s.redsync == nil {
		return func() {}, nil
	}
	mutex := s.redsync.NewMutex(fmt.Sprintf(constant.KeyCartLock, userID.String()))
	if err := mutex.Lock(); err != nil {
		return nil, errors.New("failed to acquire cart lock, please try again")
	}
	return func() { mutex.Unlock() }, nil
}

func (s *cartService) GetCart(ctx context.Context, userID uuid.UUID) (*model.CartResponse, error) {
	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.toCartResponse(cart), nil
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
		return nil, errors.New("insufficient stock")
	}

	unlock, err := s.lockCart(userID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	cart, err := s.cartRepo.GetCart(ctx, userID)
	if err != nil {
		cart = &model.Cart{
			UserID: userID,
			Items:  []model.CartItem{},
		}
	}

	found := false
	for i, item := range cart.Items {
		if item.ProductID == productID {
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

	unlock, err := s.lockCart(userID)
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
			cart.Items[i].Quantity = req.Quantity
			found = true
			break
		}
	}

	if !found {
		return nil, errors.New("item not found in cart")
	}

	cart.UpdatedAt = time.Now()
	if err := s.cartRepo.SaveCart(ctx, cart); err != nil {
		return nil, errors.New("failed to save cart")
	}

	return s.toCartResponse(cart), nil
}

func (s *cartService) RemoveItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID) (*model.CartResponse, error) {
	unlock, err := s.lockCart(userID)
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

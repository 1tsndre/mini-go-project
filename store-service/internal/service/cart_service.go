package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository"
	"github.com/go-redsync/redsync/v4"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CartService interface {
	GetCart(ctx context.Context, userID uuid.UUID) (*model.CartResponse, error)
	AddItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.AddCartItemRequest) (*model.CartResponse, error)
	UpdateItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.UpdateCartItemRequest) (*model.CartResponse, error)
	RemoveItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID) (*model.CartResponse, error)
}

type cartService struct {
	cartRepo    repository.CartRepository
	productRepo repository.ProductRepository
	locker      *redsync.Redsync
}

func NewCartService(cartRepo repository.CartRepository, productRepo repository.ProductRepository, locker *redsync.Redsync) CartService {
	return &cartService{
		cartRepo:    cartRepo,
		productRepo: productRepo,
		locker:      locker,
	}
}

// A nil locker (unit tests) skips locking.
func lockUserCart(ctx context.Context, locker *redsync.Redsync, userID uuid.UUID) (func(), error) {
	if locker == nil {
		return func() {}, nil
	}

	mutex := locker.NewMutex(fmt.Sprintf(constant.KeyCartLock, userID.String()), redsync.WithExpiry(constant.CartLockExpiry))
	if err := mutex.Lock(); err != nil {
		logger.Error(ctx, constant.LogCartLockFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgCartLockFailed}
	}
	return func() { mutex.Unlock() }, nil
}

func (s *cartService) GetCart(ctx context.Context, userID uuid.UUID) (*model.CartResponse, error) {
	cart, err := s.cartRepo.FindByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, constant.LogCartFetchFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgCartFetchFailed}
	}
	s.refreshItems(ctx, cart)
	return s.toCartResponse(cart), nil
}

// refreshItems shows the current prices, which checkout charges.
func (s *cartService) refreshItems(ctx context.Context, cart *model.Cart) {
	for i := range cart.Items {
		item := &cart.Items[i]

		product, err := s.productRepo.FindByID(ctx, item.ProductID)
		if err != nil {
			continue
		}
		item.Name = product.Name
		item.Price = product.Price
		item.ImageURL = product.ImageURL
	}
}

func (s *cartService) changeCart(ctx context.Context, userID uuid.UUID, change func(cart *model.Cart) error) (*model.CartResponse, error) {
	unlock, err := lockUserCart(ctx, s.locker, userID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	// Saving after a failed load would replace the whole cart.
	cart, err := s.cartRepo.FindByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, constant.LogCartLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgCartLoadFailed}
	}

	if err := change(cart); err != nil {
		return nil, err
	}

	s.refreshItems(ctx, cart)
	cart.UpdatedAt = time.Now()
	if err := s.cartRepo.Save(ctx, cart); err != nil {
		logger.Error(ctx, constant.LogCartSaveFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgCartSaveFailed}
	}

	return s.toCartResponse(cart), nil
}

func (s *cartService) AddItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.AddCartItemRequest) (*model.CartResponse, error) {
	product, err := s.productRepo.FindByID(ctx, productID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgProductNotFound}
	}
	if err != nil {
		logger.Error(ctx, constant.LogProductLoadFailed, err)
		return nil, &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
	}
	if product.Stock < req.Quantity {
		return nil, &Error{Code: constant.ErrCodeInsufficientStock, Message: constant.MsgInsufficientStock}
	}

	return s.changeCart(ctx, userID, func(cart *model.Cart) error {
		i := slices.IndexFunc(cart.Items, func(item model.CartItem) bool { return item.ProductID == productID })
		if i == -1 {
			cart.Items = append(cart.Items, model.CartItem{
				ProductID: productID,
				Name:      product.Name,
				Price:     product.Price,
				Quantity:  req.Quantity,
				ImageURL:  product.ImageURL,
			})
			return nil
		}
		if product.Stock < cart.Items[i].Quantity+req.Quantity {
			return &Error{Code: constant.ErrCodeInsufficientStock, Message: constant.MsgInsufficientStock}
		}
		cart.Items[i].Quantity += req.Quantity
		return nil
	})
}

func (s *cartService) UpdateItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req model.UpdateCartItemRequest) (*model.CartResponse, error) {
	return s.changeCart(ctx, userID, func(cart *model.Cart) error {
		i := slices.IndexFunc(cart.Items, func(item model.CartItem) bool { return item.ProductID == productID })
		if i == -1 {
			return &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgCartItemNotFound}
		}

		product, err := s.productRepo.FindByID(ctx, productID)
		if errors.Is(err, repository.ErrNotFound) {
			return &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgProductNotFound}
		}
		if err != nil {
			logger.Error(ctx, constant.LogProductLoadFailed, err)
			return &Error{Code: constant.ErrCodeInternal, Message: constant.MsgInternalServerError}
		}
		if product.Stock < req.Quantity {
			return &Error{Code: constant.ErrCodeInsufficientStock, Message: constant.MsgInsufficientStock}
		}
		cart.Items[i].Quantity = req.Quantity
		return nil
	})
}

func (s *cartService) RemoveItem(ctx context.Context, userID uuid.UUID, productID uuid.UUID) (*model.CartResponse, error) {
	return s.changeCart(ctx, userID, func(cart *model.Cart) error {
		i := slices.IndexFunc(cart.Items, func(item model.CartItem) bool { return item.ProductID == productID })
		if i == -1 {
			return &Error{Code: constant.ErrCodeNotFound, Message: constant.MsgCartItemNotFound}
		}
		cart.Items = slices.Delete(cart.Items, i, i+1)
		return nil
	})
}

func (s *cartService) toCartResponse(cart *model.Cart) *model.CartResponse {
	var (
		total = decimal.Zero
		items = make([]model.CartItemResponse, 0, len(cart.Items))
	)

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

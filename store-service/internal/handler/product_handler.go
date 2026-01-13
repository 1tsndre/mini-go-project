package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/pagination"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
)

type ProductHandler struct {
	service service.ProductService
}

func NewProductHandler(service service.ProductService) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var (
		req  model.CreateProductRequest
		errs []response.Error
	)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	if !decodeBody(w, r, &req) {
		return
	}

	nameChars := utf8.RuneCountInString(req.Name)
	switch {
	case req.Name == "":
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "name", Message: constant.MsgIsRequired})
	case nameChars > constant.MaxVarcharLength:
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "name", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxVarcharLength)})
	}

	descriptionChars := utf8.RuneCountInString(req.Description)
	if descriptionChars > constant.MaxDescriptionLength {
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "description", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxDescriptionLength)})
	}

	if req.Price == "" {
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "price", Message: constant.MsgIsRequired})
	}
	categoryID, err := uuid.Parse(req.CategoryID)
	switch {
	case req.CategoryID == "":
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "category_id", Message: constant.MsgIsRequired})
	case err != nil:
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "category_id", Message: constant.MsgInvalidUUID})
	}

	if len(errs) > 0 {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), errs...)
		return
	}

	product, err := h.service.CreateProduct(r.Context(), userID, categoryID, req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusCreated, product, middleware.BuildMeta(r))
}

func (h *ProductHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	var (
		q          = r.URL.Query()
		categoryID = q.Get("category_id")
		storeID    = q.Get("store_id")
		errs       []response.Error
	)

	if categoryID != "" {
		id, err := uuid.Parse(categoryID)
		if err != nil {
			errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "category_id", Message: constant.MsgInvalidUUID})
		}
		categoryID = id.String()
	}

	if storeID != "" {
		id, err := uuid.Parse(storeID)
		if err != nil {
			errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "store_id", Message: constant.MsgInvalidUUID})
		}
		storeID = id.String()
	}

	// PostgreSQL rejects NUL and invalid UTF-8.
	if search := q.Get("search"); strings.ContainsRune(search, 0) || !utf8.ValidString(search) {
		errs = append(errs, response.Error{Code: constant.ErrCodeValidation, Field: "search", Message: constant.MsgInvalidChars})
	}

	if len(errs) > 0 {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), errs...)
		return
	}

	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	page, perPage = pagination.Normalize(page, perPage)

	products, total, err := h.service.GetProducts(r.Context(), model.ProductFilter{
		CategoryID: categoryID,
		StoreID:    storeID,
		Search:     q.Get("search"),
		MinPrice:   q.Get("min_price"),
		MaxPrice:   q.Get("max_price"),
		SortBy:     q.Get("sort_by"),
		SortOrder:  q.Get("sort_order"),
		Page:       page,
		PerPage:    perPage,
	})
	if err != nil {
		serviceFailure(w, r, err)
		return
	}

	meta := middleware.BuildMeta(r)
	meta.Pagination = &response.Pagination{
		CurrentPage: page,
		PerPage:     perPage,
		TotalItems:  total,
		TotalPages:  pagination.TotalPages(total, perPage),
	}
	response.Success(w, http.StatusOK, products, meta)
}

func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "product")})
		return
	}

	product, err := h.service.GetProductByID(r.Context(), id)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, product, middleware.BuildMeta(r))
}

func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	var (
		req        model.UpdateProductRequest
		categoryID uuid.UUID
	)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "product")})
		return
	}

	if !decodeBody(w, r, &req) {
		return
	}

	nameChars := utf8.RuneCountInString(req.Name)
	if nameChars > constant.MaxVarcharLength {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "name", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxVarcharLength)})
		return
	}

	descriptionChars := utf8.RuneCountInString(req.Description)
	if descriptionChars > constant.MaxDescriptionLength {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "description", Message: fmt.Sprintf(constant.MsgMaxCharacters, constant.MaxDescriptionLength)})
		return
	}

	if req.CategoryID != "" {
		if categoryID, err = uuid.Parse(req.CategoryID); err != nil {
			response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Field: "category_id", Message: constant.MsgInvalidUUID})
			return
		}
	}

	product, err := h.service.UpdateProduct(r.Context(), userID, id, categoryID, req)
	if err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, product, middleware.BuildMeta(r))
}

func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeUnauthorized, Message: constant.MsgInvalidUser})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.Failure(w, http.StatusBadRequest, middleware.BuildMeta(r), response.Error{Code: constant.ErrCodeValidation, Message: fmt.Sprintf(constant.MsgInvalidID, "product")})
		return
	}

	if err := h.service.DeleteProduct(r.Context(), userID, id); err != nil {
		serviceFailure(w, r, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": constant.MsgProductDeleted}, middleware.BuildMeta(r))
}

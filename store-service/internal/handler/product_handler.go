package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/1tsndre/mini-go-project/pkg/response"
	"github.com/1tsndre/mini-go-project/pkg/upload"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/middleware"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/1tsndre/mini-go-project/store-service/internal/pagination"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
)

type ProductHandler struct {
	service  service.ProductService
	uploader *upload.Uploader
}

func NewProductHandler(service service.ProductService, uploader *upload.Uploader) *ProductHandler {
	return &ProductHandler{service: service, uploader: uploader}
}

func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	var req model.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	var errors []response.Error
	if req.Name == "" {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "name", "is required"))
	}
	if req.Price == "" {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "price", "is required"))
	}
	if req.CategoryID == "" {
		errors = append(errors, response.NewFieldError(constant.ErrCodeValidation, "category_id", "is required"))
	}
	if len(errors) > 0 {
		response.ValidationError(w, meta, errors)
		return
	}

	resp, err := h.service.CreateProduct(r.Context(), userID, req)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		case strings.Contains(msg, "failed"):
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		default:
			response.ErrorResponse(w, http.StatusBadRequest, meta,
				response.NewError(constant.ErrCodeValidation, msg))
		}
		return
	}

	response.Success(w, http.StatusCreated, resp, meta)
}

func (h *ProductHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))

	filter := model.ProductFilter{
		CategoryID: q.Get("category_id"),
		StoreID:    q.Get("store_id"),
		Search:     q.Get("search"),
		MinPrice:   q.Get("min_price"),
		MaxPrice:   q.Get("max_price"),
		SortBy:     q.Get("sort_by"),
		SortOrder:  q.Get("sort_order"),
		Page:       page,
		PerPage:    perPage,
	}

	products, total, err := h.service.GetProducts(r.Context(), filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, meta,
			response.NewError(constant.ErrCodeInternal, err.Error()),
		)
		return
	}

	filter.Page, filter.PerPage = pagination.Normalize(filter.Page, filter.PerPage)
	response.SuccessWithPagination(w, http.StatusOK, products, meta, &response.Pagination{
		CurrentPage: filter.Page,
		PerPage:     filter.PerPage,
		TotalItems:  total,
		TotalPages:  pagination.TotalPages(total, filter.PerPage),
	})
}

func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid product id"),
		)
		return
	}

	resp, err := h.service.GetProductByID(r.Context(), id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, meta,
			response.NewError(constant.ErrCodeNotFound, err.Error()),
		)
		return
	}

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid product id"),
		)
		return
	}

	var req model.UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid request body"),
		)
		return
	}

	resp, err := h.service.UpdateProduct(r.Context(), userID, id, req)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		case strings.Contains(msg, "forbidden"):
			response.ErrorResponse(w, http.StatusForbidden, meta,
				response.NewError(constant.ErrCodeForbidden, msg))
		case strings.Contains(msg, "failed"):
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		default:
			response.ErrorResponse(w, http.StatusBadRequest, meta,
				response.NewError(constant.ErrCodeValidation, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, resp, meta)
}

func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid product id"),
		)
		return
	}

	if err := h.service.DeleteProduct(r.Context(), userID, id); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		case strings.Contains(msg, "forbidden"):
			response.ErrorResponse(w, http.StatusForbidden, meta,
				response.NewError(constant.ErrCodeForbidden, msg))
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "product deleted"}, meta)
}

func (h *ProductHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	meta := middleware.BuildMeta(r)

	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, meta,
			response.NewError(constant.ErrCodeUnauthorized, "invalid user"),
		)
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "invalid product id"),
		)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, "image file is required"),
		)
		return
	}
	defer file.Close()

	path, err := h.uploader.Upload(file, header, "products")
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, meta,
			response.NewError(constant.ErrCodeValidation, err.Error()),
		)
		return
	}

	resp, err := h.service.UpdateImage(r.Context(), userID, id, path)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "not found"):
			response.ErrorResponse(w, http.StatusNotFound, meta,
				response.NewError(constant.ErrCodeNotFound, msg))
		case strings.Contains(msg, "forbidden"):
			response.ErrorResponse(w, http.StatusForbidden, meta,
				response.NewError(constant.ErrCodeForbidden, msg))
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, meta,
				response.NewError(constant.ErrCodeInternal, msg))
		}
		return
	}

	response.Success(w, http.StatusOK, resp, meta)
}

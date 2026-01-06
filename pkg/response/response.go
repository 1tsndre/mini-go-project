package response

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Data   interface{} `json:"data,omitempty"`
	Meta   *Meta       `json:"meta"`
	Errors []Error     `json:"errors,omitempty"`
}

type Meta struct {
	RequestID  string      `json:"request_id"`
	Timestamp  string      `json:"timestamp"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type Pagination struct {
	CurrentPage int   `json:"current_page"`
	PerPage     int   `json:"per_page"`
	TotalItems  int64 `json:"total_items"`
	TotalPages  int64 `json:"total_pages"`
}

type Error struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

const contentTypeJSON = "application/json"

func writeJSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func Success(w http.ResponseWriter, status int, data interface{}, meta *Meta) {
	writeJSON(w, status, Response{
		Data: data,
		Meta: meta,
	})
}

func SuccessWithPagination(w http.ResponseWriter, status int, data interface{}, meta *Meta, pagination *Pagination) {
	meta.Pagination = pagination
	writeJSON(w, status, Response{
		Data: data,
		Meta: meta,
	})
}

func ErrorResponse(w http.ResponseWriter, status int, meta *Meta, errors ...Error) {
	writeJSON(w, status, Response{
		Meta:   meta,
		Errors: errors,
	})
}

func ValidationError(w http.ResponseWriter, meta *Meta, errors []Error) {
	writeJSON(w, http.StatusBadRequest, Response{
		Meta:   meta,
		Errors: errors,
	})
}

func NewError(code, message string) Error {
	return Error{Code: code, Message: message}
}

func NewFieldError(code, field, message string) Error {
	return Error{Code: code, Field: field, Message: message}
}

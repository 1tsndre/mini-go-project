package response

import (
	"encoding/json"
	"net/http"

	"github.com/1tsndre/mini-go-project/pkg/constant"
)

type Response struct {
	Data   any     `json:"data,omitempty"`
	Meta   *Meta   `json:"meta"`
	Errors []Error `json:"errors,omitempty"`
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

func writeJSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set(constant.HeaderContentType, constant.ContentTypeJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func Success(w http.ResponseWriter, status int, data any, meta *Meta) {
	writeJSON(w, status, Response{Data: data, Meta: meta})
}

func Failure(w http.ResponseWriter, status int, meta *Meta, errors ...Error) {
	writeJSON(w, status, Response{Meta: meta, Errors: errors})
}

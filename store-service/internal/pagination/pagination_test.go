package pagination

import (
	"math"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/stretchr/testify/assert"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		page, perPage         int
		wantPage, wantPerPage int
	}{
		{page: 0, perPage: 0, wantPage: 1, wantPerPage: 10},
		{page: -1, perPage: -1, wantPage: 1, wantPerPage: 10},
		{page: 3, perPage: 20, wantPage: 3, wantPerPage: 20},
		{page: math.MaxInt, perPage: 999, wantPage: constant.MaxPage, wantPerPage: constant.MaxPerPage},
	}

	for _, tt := range tests {
		page, perPage := Normalize(tt.page, tt.perPage)
		assert.Equal(t, tt.wantPage, page)
		assert.Equal(t, tt.wantPerPage, perPage)
	}
}

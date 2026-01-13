package model

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestParsePrice(t *testing.T) {
	for _, s := range []string{"0", "-1", "100.50", "9999999999999.99"} {
		price, err := ParsePrice(s)
		assert.NoError(t, err, s)
		assert.True(t, decimal.RequireFromString(s).Equal(price), s)
	}

	for _, s := range []string{"", "cheap", "1e3", "1E1000000000", "1e-1000000000", strings.Repeat("9", 33)} {
		_, err := ParsePrice(s)
		assert.Error(t, err, s)
	}
}

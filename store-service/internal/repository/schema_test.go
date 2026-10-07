package repository

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The SQL in this package is written by hand. These tests check the column lists
// and the models' db tags against the schema in migrations/, so a misspelled
// column fails here instead of at runtime, without needing a database.

var (
	createTableRe = regexp.MustCompile(`(?s)CREATE TABLE (\w+) \((.*?)\n\);`)
	addColumnRe   = regexp.MustCompile(`ALTER TABLE (\w+) ADD COLUMN (\w+)`)
)

func migratedSchema(t *testing.T) map[string]map[string]bool {
	t.Helper()
	files, err := filepath.Glob("../../../migrations/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	sort.Strings(files)

	schema := make(map[string]map[string]bool)
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)

		for _, m := range createTableRe.FindAllStringSubmatch(string(data), -1) {
			columns := make(map[string]bool)
			for _, line := range strings.Split(m[2], "\n") {
				fields := strings.Fields(line)
				if len(fields) == 0 {
					continue
				}
				name := strings.ToLower(fields[0])
				// Table constraints, e.g. UNIQUE(user_id, product_id), are not columns.
				if strings.HasPrefix(name, "unique") || name == "primary" || name == "foreign" ||
					name == "check" || name == "constraint" {
					continue
				}
				columns[name] = true
			}
			schema[m[1]] = columns
		}
		for _, m := range addColumnRe.FindAllStringSubmatch(string(data), -1) {
			require.Contains(t, schema, m[1], "%s alters a table no migration creates", file)
			schema[m[1]][m[2]] = true
		}
	}
	return schema
}

func TestColumnListsMatchSchema(t *testing.T) {
	schema := migratedSchema(t)
	lists := map[string]string{
		"users":       userColumns,
		"stores":      storeColumns,
		"categories":  categoryColumns,
		"products":    productColumns,
		"reviews":     reviewColumns,
		"orders":      orderColumns,
		"order_items": orderItemColumns,
		"payments":    paymentColumns,
	}

	for table, list := range lists {
		columns, ok := schema[table]
		if !assert.True(t, ok, "table %s is not in the migrations", table) {
			continue
		}
		for _, column := range strings.Split(list, ", ") {
			assert.True(t, columns[column], "column %s.%s does not exist", table, column)
		}
	}
}

func TestModelTagsMatchSchema(t *testing.T) {
	schema := migratedSchema(t)
	models := []struct {
		model  any
		tables []string // each tag must be a column of one of these tables
	}{
		{model: model.User{}, tables: []string{"users"}},
		{model: model.Store{}, tables: []string{"stores"}},
		{model: model.Category{}, tables: []string{"categories"}},
		{model: model.Product{}, tables: []string{"products"}},
		{model: model.Review{}, tables: []string{"reviews"}},
		{model: model.Order{}, tables: []string{"orders"}},
		{model: model.OrderItem{}, tables: []string{"order_items"}},
		{model: model.Payment{}, tables: []string{"payments"}},
		{model: model.CartItemDB{}, tables: []string{"cart_items"}},
		// The cart is read by joining cart_items with products.
		{model: model.CartItem{}, tables: []string{"cart_items", "products"}},
	}
	// Columns computed by a query rather than stored in the model's table.
	computed := map[string]bool{"user_name": true}

	for _, m := range models {
		typ := reflect.TypeOf(m.model)
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			tag, ok := field.Tag.Lookup("db")
			if !assert.True(t, ok, "%s.%s has no db tag", typ.Name(), field.Name) || tag == "-" || computed[tag] {
				continue
			}
			found := false
			for _, table := range m.tables {
				found = found || schema[table][tag]
			}
			assert.True(t, found, "%s.%s maps to %q, which is not a column of %v", typ.Name(), field.Name, tag, m.tables)
		}
	}
}

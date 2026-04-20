package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"tars/internal/api/dto"
)

const (
	defaultListPage  = 1
	defaultListLimit = 20
	maxListLimit     = 100
)

type listQuery struct {
	Page      int
	Limit     int
	Query     string
	SortBy    string
	SortOrder string
}

func parseListQuery(r *http.Request) listQuery {
	values := r.URL.Query()
	page := parsePositiveInt(values.Get("page"), defaultListPage)
	limit := parsePositiveInt(values.Get("limit"), defaultListLimit)
	if limit > maxListLimit {
		limit = maxListLimit
	}
	sortOrder := strings.ToLower(strings.TrimSpace(values.Get("sort_order")))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	return listQuery{
		Page:      page,
		Limit:     limit,
		Query:     strings.TrimSpace(values.Get("q")),
		SortBy:    strings.TrimSpace(values.Get("sort_by")),
		SortOrder: sortOrder,
	}
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func paginateItems[T any](items []T, query listQuery) ([]T, dto.ListPage) {
	total := len(items)
	start := (query.Page - 1) * query.Limit
	if start > total {
		start = total
	}
	end := start + query.Limit
	if end > total {
		end = total
	}
	pageItems := make([]T, 0, end-start)
	pageItems = append(pageItems, items[start:end]...)
	return pageItems, dto.ListPage{
		Page:      query.Page,
		Limit:     query.Limit,
		Total:     total,
		HasNext:   end < total,
		Query:     query.Query,
		SortBy:    query.SortBy,
		SortOrder: query.SortOrder,
	}
}

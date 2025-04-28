package model

import (
	"math"
	"strings"

	"github.com/kinkando/pharma-sheet-service/pkg/util"
)

type Pagination struct {
	Limit  uint64  `json:"limit" query:"limit"`
	Offset uint64  `json:"offset" query:"offset"`
	Page   uint64  `json:"page" query:"page"`
	Sort   *string `json:"sort" query:"sort"`
	Search string  `json:"-" query:"search"`
}

func (p *Pagination) AssignDefault() {
	if p.Limit == 0 {
		p.Limit = 10
	}
	if p.Page > 0 {
		p.Offset = (p.Page - 1) * p.Limit
	}
}

func (p *Pagination) SortBy(sortBy string) string {
	if p.Sort == nil || *p.Sort == "" {
		return sortBy
	}
	if sorts := strings.Split(*p.Sort, " "); p.Sort != nil && *p.Sort != "" && len(sorts) == 2 {
		sortBy = util.CamelToSnake(strings.ReplaceAll(sorts[0], "ID", "Id")) + " " + sorts[1]
	}
	if !strings.HasSuffix(strings.ToUpper(sortBy), " ASC") && !strings.HasSuffix(strings.ToUpper(sortBy), " DESC") {
		sortBy = strings.Split(*p.Sort, " ")[0] + " ASC"
	}
	return sortBy
}

type PagingWithMetadata[T any] struct {
	Data     []T      `json:"data,omitempty"`
	Metadata Metadata `json:"metadata"`
}

type Metadata struct {
	Limit       uint64 `json:"limit"`
	Offset      uint64 `json:"offset"`
	TotalItem   uint64 `json:"totalItem"`
	TotalPage   uint64 `json:"totalPage"`
	CurrentPage uint64 `json:"currentPage"`
}

func PaginationResponse[T any](data []T, paging Pagination, total uint64) PagingWithMetadata[T] {
	totalPage := uint64(math.Ceil(float64(total) / float64(paging.Limit)))
	currentPage := paging.Offset/paging.Limit + 1
	if totalPage <= 0 {
		currentPage = 0
	}
	return PagingWithMetadata[T]{
		Data: data,
		Metadata: Metadata{
			Limit:       paging.Limit,
			Offset:      paging.Offset,
			TotalItem:   total,
			TotalPage:   totalPage,
			CurrentPage: currentPage,
		},
	}
}

type PaginationResult[T any] struct {
	Data  []T    `bson:"data"`
	Total uint64 `bson:"total"`
}

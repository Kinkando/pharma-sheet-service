package model

import "math"

type Pagination struct {
	Limit  uint64  `json:"limit" query:"limit"`
	Offset uint64  `json:"offset" query:"offset"`
	Page   uint64  `json:"page" query:"page"`
	Sort   *string `json:"sort" query:"sort"`
}

func (p *Pagination) AssignDefault() {
	if p.Limit == 0 {
		p.Limit = 10
	}
	if p.Page > 0 {
		p.Offset = (p.Page - 1) * p.Limit
	}
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

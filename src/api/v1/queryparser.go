package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/errors"

	"github.com/gin-gonic/gin"
)

type ProductListQuery struct {
	Limit int    `form:"limit" binding:"omitempty,min=1,max=1000"`
	IDs   []uint `form:"ids" collection_format:"csv" binding:"omitempty,max=1000,dive,min=1"`
}

type ProductSortQuery struct {
	Sort  string `form:"sort" binding:"omitempty,oneof=product_name expire_at created_at"`
	Order string `form:"order" binding:"omitempty,oneof=asc desc"`
}

type ArchiveOnlyQuery struct {
	ArchiveOnly bool `form:"archiveOnly"`
}

type ProductSearchQuery struct {
	QueryParam string `form:"queryParam" binding:"required"`
	QueryValue string `form:"queryValue" binding:"required"`
	Sort       string `form:"sort" binding:"omitempty,oneof=product_name expire_at created_at scanned_at notified_at barcode"`
	Order      string `form:"order" binding:"omitempty,oneof=asc desc"`
}

func ParseProductListQuery(ctx *gin.Context) (ProductListQuery, bool) {
	var q ProductListQuery
	if ctx.Request.URL == nil {
		q.Limit = 100
		return q, true
	}
	if err := ctx.ShouldBindQuery(&q); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidQueryParameter)
		return ProductListQuery{}, false
	}
	if q.Limit == 0 {
		q.Limit = 100
	}
	return q, true
}

func ParseProductSortQuery(ctx *gin.Context) (ProductSortQuery, bool) {
	if ctx.Request.URL == nil {
		return ProductSortQuery{}, true
	}
	var q ProductSortQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidQueryParameter)
		return ProductSortQuery{}, false
	}
	return q, true
}

func ParseArchiveOnly(ctx *gin.Context) (bool, bool) {
	if ctx.Request.URL == nil {
		return false, true
	}
	var q ArchiveOnlyQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidQueryParameter)
		return false, false
	}
	return q.ArchiveOnly, true
}

func ParseProductSearchQuery(ctx *gin.Context) (ProductSearchQuery, bool) {
	if ctx.Request.URL == nil {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidQueryParameter)
		return ProductSearchQuery{}, false
	}
	var q ProductSearchQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidQueryParameter)
		return ProductSearchQuery{}, false
	}
	return q, true
}

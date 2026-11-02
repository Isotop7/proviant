package v1

import (
	"net/http"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// ProductListQuery is the window a product list is read in: how many products
// at most, how many to skip before the first one, and — for a client that
// already knows the ids — the products to read instead.
type ProductListQuery struct {
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=1000"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	IDs    []uint `form:"ids" collection_format:"csv" binding:"omitempty,max=1000,dive,min=1"`
}

type ProductSortQuery struct {
	Sort  string `form:"sort" binding:"omitempty,oneof=product_name expire_at created_at effective_expire_at"`
	Order string `form:"order" binding:"omitempty,oneof=asc desc"`
}

type ArchiveOnlyQuery struct {
	ArchiveOnly bool `form:"archiveOnly"`
}

// ProductSearchQuery is a search: the field and the value to match, the order
// of the matches and the window they are read in. A limit of zero means every
// match, which is what a search returned before it could be paged.
type ProductSearchQuery struct {
	QueryParam string `form:"queryParam" binding:"required"`
	QueryValue string `form:"queryValue" binding:"required"`
	Sort       string `form:"sort" binding:"omitempty,oneof=product_name expire_at created_at scanned_at notified_at barcode effective_expire_at"`
	Order      string `form:"order" binding:"omitempty,oneof=asc desc"`
	Limit      int    `form:"limit" binding:"omitempty,min=0,max=1000"`
	Offset     int    `form:"offset" binding:"omitempty,min=0"`
}

// offsetWithinBound reports whether an offset is one a product list can be
// read at. The bound is a constant rather than a tag, because a binding tag
// cannot carry one.
func offsetWithinBound(offset int) bool {
	return offset >= 0 && offset <= util.MaxProductListOffset
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
	if !offsetWithinBound(q.Offset) {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidQueryParameter)
		return ProductListQuery{}, false
	}
	if q.Limit == 0 {
		q.Limit = 100
	}
	return q, true
}

// ParseProductSortQuery reads the sort key and direction of a product list.
// The default is the effective expiry, the order the list endpoint has always
// returned, so naming it does not reorder the clients that ask for nothing.
func ParseProductSortQuery(ctx *gin.Context) (ProductSortQuery, bool) {
	if ctx.Request.URL == nil {
		return ProductSortQuery{Sort: database.SortEffectiveExpiry, Order: "asc"}, true
	}
	var q ProductSortQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidQueryParameter)
		return ProductSortQuery{}, false
	}
	if q.Sort == "" {
		q.Sort = database.SortEffectiveExpiry
	}
	if q.Order == "" {
		q.Order = "asc"
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
	if !offsetWithinBound(q.Offset) {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidQueryParameter)
		return ProductSearchQuery{}, false
	}
	return q, true
}

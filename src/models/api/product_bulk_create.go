package api

import "time"

// BulkCreateProductItem is one scanned product queued by the batch scan mode
// on the client. ProductName is what the client already fetched from the Open
// Food Facts proxy; the server re-resolves missing names with the import name
// resolver.
type BulkCreateProductItem struct {
	Barcode           string    `json:"barcode"`
	ProductName       string    `json:"productName,omitempty"`
	ExpireAt          time.Time `json:"expireAt"`
	Amount            int       `json:"amount"`
	StorageLocationID *uint     `json:"storageLocationId"`
}

// BulkCreateProductsAPIModel is the request body for POST /api/v1/products/bulk.
type BulkCreateProductsAPIModel struct {
	Items []BulkCreateProductItem `json:"items"`
}

// BulkCreateItemError describes why a single queued item was rejected. Index is
// the 0-based position of the item in the request, so the client can map the
// reason back to its queue entry.
type BulkCreateItemError struct {
	Index   int    `json:"index"`
	Barcode string `json:"barcode,omitempty"`
	Reason  string `json:"reason"`
}

// BulkCreateResponse is the response body for POST /api/v1/products/bulk.
// Created and Failed always sum to the number of submitted items. Errors is
// never nil so it serialises as [] rather than null; the frontend iterates it
// unconditionally.
type BulkCreateResponse struct {
	Message string                `json:"message,omitempty"`
	Created int                   `json:"created"`
	Failed  int                   `json:"failed"`
	Errors  []BulkCreateItemError `json:"errors"`
}

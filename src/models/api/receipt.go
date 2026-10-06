package api

// ReceiptItem is one line item extracted from a receipt photo
type ReceiptItem struct {
	Name   string   `json:"name"`
	Amount int      `json:"amount"`
	Unit   string   `json:"unit"`
	Price  *float64 `json:"price,omitempty"`
}

// ReceiptScanResponse is the response of POST /api/v1/products/scan-receipt.
// An empty Items slice means the scan succeeded but nothing was recognized.
type ReceiptScanResponse struct {
	Items []ReceiptItem `json:"items"`
}

// BulkProductDraft is one product to create via the bulk endpoint
type BulkProductDraft struct {
	ProductName       string   `json:"productName"`
	Barcode           string   `json:"barcode"`
	Amount            int      `json:"amount"`
	Unit              string   `json:"unit"`
	ExpireAt          string   `json:"expireAt"`
	Categories        string   `json:"categories"`
	StorageLocationID *uint    `json:"storageLocationId"`
	PriceOverride     *float64 `json:"priceOverride,omitempty"`
	IsPrivate         bool     `json:"isPrivate"`
}

// BulkCreateRequest is the body of POST /api/v1/products/bulk.
// Per-item processing: partial success is allowed and reported.
type BulkCreateRequest struct {
	Items []BulkProductDraft `json:"items"`
}

// BulkItemStatus values for BulkItemResult.Status
const (
	BulkItemStatusCreated = "created"
	BulkItemStatusFailed  = "failed"
)

// BulkItemResult is the per-item outcome of a bulk create
type BulkItemResult struct {
	Index     int    `json:"index"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	ProductID *uint  `json:"productId,omitempty"`
}

// BulkCreateResponse reports the outcome of every submitted item
type BulkCreateResponse struct {
	Results []BulkItemResult `json:"results"`
}

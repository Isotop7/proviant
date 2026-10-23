package api

// ReceiptItem is one line item extracted from a receipt photo. Quantity is the
// number of packs the line stands for and PackSize is the size of one pack as
// the receipt words it.
type ReceiptItem struct {
	Name     string   `json:"name"`
	Quantity int      `json:"quantity"`
	PackSize string   `json:"packSize"`
	Price    *float64 `json:"price,omitempty"`
}

// ReceiptScanResponse is the response of POST /api/v1/products/scan-receipt.
// An empty Items slice means the scan succeeded but nothing was recognized.
type ReceiptScanResponse struct {
	Items []ReceiptItem `json:"items"`
}

// BulkProductDraft is one product to create via the bulk endpoint
type BulkProductDraft struct {
	ProductName string `json:"productName"`
	Barcode     string `json:"barcode"`
	// Quantity is the number of packs the product stands for.
	Quantity int `json:"quantity"`
	// PackSize is the size of one pack as text ("1 l"), the display metadata
	// the receipt read off the line.
	PackSize string `json:"packSize"`
	// ExpireAt is the expiry date the receipt carries, as an RFC 3339 date.
	ExpireAt   string `json:"expireAt"`
	Categories string `json:"categories"`
	// StorageLocationID is the location the product is stored in.
	StorageLocationID *uint `json:"storageLocationId"`
	// Price is the price of the whole line, as the receipt prints it. The
	// price of one pack is the line price divided by the count.
	Price *float64 `json:"price,omitempty"`
	// PriceOverride is the price of one pack, as a draft written before
	// products were counted in packs carried it. A draft that carries only
	// this field is read as the price of one pack; Price wins when both are
	// present.
	PriceOverride *float64 `json:"priceOverride,omitempty"`
	// Amount and Unit carry the net content of one pack, as a draft written
	// before products were counted in packs carried it. A queued draft from
	// before that change still carries it. A draft that carries only these is
	// read as one pack of that net content.
	Amount    int    `json:"amount,omitempty"`
	Unit      string `json:"unit,omitempty"`
	IsPrivate bool   `json:"isPrivate"`
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
	// Terminal marks a failed item whose reason is a property of the draft
	// itself, so resubmitting it unchanged fails identically. A caller that
	// retries asynchronously should surface such an item to the user instead of
	// spending its retry budget on it.
	Terminal bool `json:"terminal,omitempty"`
}

// BulkCreateResponse reports the outcome of every submitted item
type BulkCreateResponse struct {
	Results []BulkItemResult `json:"results"`
}

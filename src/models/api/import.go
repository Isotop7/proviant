package api

// ImportRowError describes why a single CSV row was rejected. Row is the
// 1-based line number in the uploaded file, counting the header, so it
// matches what a spreadsheet shows. A row whose quoted cell spans several
// physical lines is reported at the line it ends on.
type ImportRowError struct {
	Row     int    `json:"row"`
	Name    string `json:"name,omitempty"`
	Barcode string `json:"barcode,omitempty"`
	Reason  string `json:"reason"`
}

// ImportProductsResponse is the response body for POST /api/v1/products/import.
// Errors is never nil so it serialises as [] rather than null; the frontend
// iterates it unconditionally.
type ImportProductsResponse struct {
	Message          string           `json:"message,omitempty"`
	TotalRows        int              `json:"totalRows"`
	Imported         int              `json:"imported"`
	Failed           int              `json:"failed"`
	Errors           []ImportRowError `json:"errors"`
	CreatedLocations []string         `json:"createdLocations,omitempty"`
}

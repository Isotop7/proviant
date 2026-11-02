package api

type ActivityLogResponse struct {
	Activities []ActivityEntry `json:"activities"`
	Total      int             `json:"total"`
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
}

type ActivityEntry struct {
	UserID      *uint  `json:"userId"`
	UserName    string `json:"userName"`
	Action      string `json:"action"`
	ProductID   uint   `json:"productId"`
	ProductName string `json:"productName"`
	Quantity    int    `json:"quantity"`
	// WasteCause is why the action wasted the product. It is empty for every
	// action but a waste that carried a reason.
	WasteCause string `json:"wasteCause"`
	Timestamp  string `json:"timestamp"`
}

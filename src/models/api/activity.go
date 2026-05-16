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
	Timestamp   string `json:"timestamp"`
}
package api

type CreateTokenRequest struct {
	Name      string  `json:"name" binding:"required"`
	ExpiresAt *string `json:"expiresAt"`
	Scopes    string  `json:"scopes"`
}

type TokenResponse struct {
	ID         uint    `json:"id"`
	Name       string  `json:"name"`
	LastUsedAt *string `json:"lastUsedAt"`
	ExpiresAt  *string `json:"expiresAt"`
	Scopes     string  `json:"scopes"`
	CreatedAt  string  `json:"createdAt"`
}

type CreateTokenResponse struct {
	Token     string  `json:"token"`
	Name      string  `json:"name"`
	ExpiresAt *string `json:"expiresAt"`
	Scopes    string  `json:"scopes"`
}

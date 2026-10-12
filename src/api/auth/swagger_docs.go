package auth

// LoginResponse mirrors the token payload emitted by the gin-jwt login and
// refresh handlers; the handlers themselves live in the gin-jwt middleware
// package, so the routes are documented here instead.
type LoginResponse struct {
	Code   int    `json:"code"`
	Expire string `json:"expire"`
	Token  string `json:"token"`
}

// swaggerLogin documents the gin-jwt login route mounted by the router.
// @Summary      Authenticates a user
// @Description  Issues a JWT access token for valid credentials. The handler is provided by the gin-jwt middleware
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials body authentication.Login true "Credentials"
// @Success      200 {object} LoginResponse
// @Failure      401 {object} api.APIResponse
// @Failure      429 {object} api.APIResponse
// @Router       /auth/login [post]
//
//nolint:unused // swag-only documentation stub, never called
func swaggerLogin() {}

// swaggerRefreshToken documents the gin-jwt refresh route mounted by the router.
// @Summary      Refreshes the JWT token
// @Description  Issues a new JWT access token from a valid, unexpired token. The handler is provided by the gin-jwt middleware
// @Tags         auth
// @Produce      json
// @Success      200 {object} LoginResponse
// @Failure      401 {object} api.APIResponse
// @Router       /auth/refresh_token [get]
//
//nolint:unused // swag-only documentation stub, never called
func swaggerRefreshToken() {}

// static implements "constants" used in proviant
package static

import "time"

const (
	// Version is the current proviant version
	Version = "v0.15.0"

	// TokenRealm is the realm of tokens
	TokenRealm = "proviant"

	// TokenIdentityKey is the name of identity key in tokens
	TokenIdentityKey = "id"

	// TokenUsernameKey is the name of username key in tokens
	TokenUsernameKey = "username"

	// TokenJTIKey is the name of JTI key in tokens
	TokenJTIKey = "jti"

	// TokenHeadName is the name of authentication header in token
	TokenHeadName = "Bearer"

	// TokenLookup is the configuration for value lookup in token
	TokenLookup = "header: Authorization, query: token, cookie: jwt"

	// BarcodeDecodingTimeout is the timeout of the decoding operation
	BarcodeDecodingTimeout = 5 * time.Second

	// RequestIDHeader is the header name for request ID
	RequestIDHeader = "X-Request-ID"
)

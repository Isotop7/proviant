// static implements "constants" used in proviant
package static

import "time"

var (
	// Realm of tokens
	TokenRealm = "proviant"

	// Name of identity key in tokens
	TokenIdentityKey = "id"

	// Name of username key in tokens
	TokenUsernameKey = "username"

	// Name of JTI key in tokens
	TokenJTIKey = "jti"

	// Name of authentication header in token
	TokenHeadName = "Bearer"

	// Configuration for value lookup in token
	TokenLookup = "header: Authorization, query: token, cookie: jwt"

	// BarcodeDecodingTimeout is the timoeut of the decoding operation in seconds
	BarcodeDecodingTimeout = time.Second * time.Duration(5)

	// RequestIDHeader is the header name for request ID
	RequestIDHeader = "X-Request-ID"

	// RequestIDContextKey is the context key for request ID
	RequestIDContextKey = "requestID"

	// UserIDContextKey is the context key for user ID (for logger enrichment)
	UserIDContextKey = "userID"
)

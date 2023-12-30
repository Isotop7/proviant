// static implements "constants" used in expiro
package static

var (
	// Realm of tokens
	TokenRealm = "expiro"

	// Name of identity key in tokens
	TokenIdentityKey = "id"

	// Name of username key in tokens
	TokenUsernameKey = "username"

	// Name of authentication header in token
	TokenHeadName = "Bearer"

	// Configuration for value lookup in token
	TokenLookup = "header: Authorization, query: token, cookie: jwt"
)

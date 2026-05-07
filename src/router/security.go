package router

import (
	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/gin-gonic/gin"
	"github.com/unrolled/secure"
)

func SecurityHeadersMiddleware(proviantConfig *configuration.ProviantConfiguration) gin.HandlerFunc {
	contentSecurityPolicy := proviantConfig.Server.SecurityHeaders.ContentSecurityPolicy
	if contentSecurityPolicy == "" {
		contentSecurityPolicy = "default-src 'self'; script-src 'self' $NONCE; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:;"
	}

	secureMiddleware := secure.New(secure.Options{
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		FrameDeny:             true,
		STSSeconds:            31536000,
		STSIncludeSubdomains:  true,
		STSPreload:            true,
		ContentSecurityPolicy: contentSecurityPolicy,
	})

	return func(c *gin.Context) {
		nonce, err := secureMiddleware.ProcessAndReturnNonce(c.Writer, c.Request)
		if err != nil {
			c.Next()
			return
		}
		c.Set("cspNonce", nonce)
		c.Next()
	}
}

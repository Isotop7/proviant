package router

import (
	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/gin-gonic/gin"
	"github.com/unrolled/secure"
)

func SecurityHeadersMiddleware(proviantConfig *configuration.ProviantConfiguration) gin.HandlerFunc {
	csp := proviantConfig.Server.SecurityHeaders.ContentSecurityPolicy
	if csp == "" {
		csp = "default-src 'self'"
	}

	secureMiddleware := secure.New(secure.Options{
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		FrameDeny:             true,
		STSSeconds:            31536000,
		STSIncludeSubdomains:  true,
		STSPreload:            true,
		ContentSecurityPolicy: csp,
	})

	return func(c *gin.Context) {
		_ = secureMiddleware.Process(c.Writer, c.Request)
		c.Next()
	}
}

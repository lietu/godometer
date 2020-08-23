package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/unrolled/secure"
)

var ErrAccessDenied = errors.New("access denied")

func SecurityMiddleware(dev bool) gin.HandlerFunc {
	// Maybe should add Feature-Policy, and Expect-CT
	secureMiddleware := secure.New(secure.Options{
		SSLRedirect:           !dev,
		SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
		STSSeconds:            31536000,
		STSIncludeSubdomains:  true,
		STSPreload:            true,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "img-src 'self'; style-src 'self' fonts.googleapis.com maxcdn.bootstrapcdn.com 'unsafe-inline' blob:; script-src 'self'; connect-src 'self'; font-src fonts.gstatic.com maxcdn.bootstrapcdn.com",
		ReferrerPolicy:        "same-origin",
		// FeaturePolicy: "autoplay 'none'; camera 'none'; display-capture 'none'; document-domain 'none';",
	})

	return func(c *gin.Context) {
		err := secureMiddleware.Process(c.Writer, c.Request)
		if err != nil {
			c.Abort()
			return
		}

		// Avoid header rewrite if response is a redirection.
		if status := c.Writer.Status(); status > 300 && status < 399 {
			c.Abort()
		}
	}
}

func AuthRequired(apiAuth string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.Request.Header.Get("Authorization")
		if auth != apiAuth {
			_ = c.AbortWithError(http.StatusForbidden, ErrAccessDenied)
			return
		}

		c.Next()
	}
}

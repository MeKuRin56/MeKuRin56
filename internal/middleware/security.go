package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds standard web security response headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")
		// Prevent clickjacking — only allow same origin iframes
		c.Header("X-Frame-Options", "SAMEORIGIN")
		// Legacy XSS filter (modern browsers use CSP instead)
		c.Header("X-XSS-Protection", "1; mode=block")
		// Don't leak full URL to external sites
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		// Disable unnecessary browser APIs
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// Enforce HTTPS in modern browsers (HSTS)
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// Content Security Policy — blocks inline scripts from other origins,
		// allows Bootstrap CDN, Bootstrap Icons CDN, and Google Fonts
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://fonts.googleapis.com; "+
				"font-src 'self' https://cdn.jsdelivr.net https://fonts.gstatic.com; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'self'; "+
				"base-uri 'self'; "+
				"form-action 'self';",
		)
		c.Next()
	}
}

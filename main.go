package main

import (
	"log"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration and initialize DynamoDB
	posts, dynamoClient, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// Get environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	sessionSecret := os.Getenv("SESSION_SECRET")
	adminUsername := os.Getenv("ADMIN_USERNAME")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	totpSecret := os.Getenv("TOTP_SECRET")

	// Initialize Gin router
	r := gin.Default()
	handler := NewBlogHandler(dynamoClient, &posts)

	// Set up session middleware
	store := cookie.NewStore([]byte(sessionSecret))
	r.Use(sessions.Sessions("mysession", store))

	// Load templates
	r.LoadHTMLGlob("templates/*")

	// Define routes
	r.GET("/", handler.homeHandler)
	r.GET("/login", loginPageHandler)
	r.GET("/totp", totpPageHandler)
	r.POST("/auth/totp", func(c *gin.Context) {
		totpHandler(c, totpSecret)
	})
	r.POST("/auth/login", func(c *gin.Context) {
		loginHandler(c, adminUsername, adminPassword)
	})
	r.GET("/auth/logout", logoutHandler)
	r.GET("/write", authMiddleware, writePageHandler)
	r.POST("/write", authMiddleware, handler.writePostHandler)
  r.GET("/write/:id", authMiddleware, handler.editPostHandler)
	r.DELETE("/posts/:id", authMiddleware, handler.deletePostHandler)
  r.GET("/calendar", calendarHandler)

	// Start server
	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}







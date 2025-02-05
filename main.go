package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/pquerna/otp/totp"
)

type BlogPost struct {
	Title   string
	Content string
}

var blogPosts = []BlogPost{
	{Title: "First Post", Content: "This is the content of the first post."},
	{Title: "Second Post", Content: "This is the content of the second post."},
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Get environment variables
	port := os.Getenv("PORT")
	sessionSecret := os.Getenv("SESSION_SECRET")
	adminUsername := os.Getenv("ADMIN_USERNAME")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	totpSecret := os.Getenv("TOTP_SECRET")

	// Create a new Gin router
	r := gin.Default()

	// Set up session middleware
	store := cookie.NewStore([]byte(sessionSecret)) // Use SESSION_SECRET from .env
	r.Use(sessions.Sessions("mysession", store))

	// Load HTML templates from the "templates" directory
	r.LoadHTMLGlob("templates/*")

	// Define routes
	r.GET("/", homeHandler)
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
	r.POST("/write", authMiddleware, writePostHandler)

	// Start the server
	r.Run(":" + port)
}

func homeHandler(c *gin.Context) {
	// Render the "index.html" template with blog posts
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "My Blog",
		"posts": blogPosts,
		"user":  sessions.Default(c).Get("loggedIn"),
	})
}

func loginPageHandler(c *gin.Context) {
	// Render the login page
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login",
	})
}

func totpPageHandler(c *gin.Context) {
	// Render the totp page
	c.HTML(http.StatusOK, "totp.html", gin.H{
		"title": "TOTP",
	})
}

func loginHandler(c *gin.Context, adminUsername, adminPassword string) {
	// Simulate user authentication
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == adminUsername && password == adminPassword {
		session := sessions.Default(c)
		session.Set("loggedIn", true)
		c.Redirect(http.StatusFound, "/totp")
	} else {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"title":   "Login",
			"message": "Invalid username or password",
		})
	}
}

func totpHandler(c *gin.Context, totpSecret string) {
	// Simulate TOTP authentication
	totpCode := c.PostForm("code")
	valid := totp.Validate(totpCode, totpSecret)
	if valid {
		session := sessions.Default(c)
		session.Set("loggedIn", true)
		session.Save()
		c.Redirect(http.StatusFound, "/")
	} else {
		c.HTML(http.StatusOK, "totp.html", gin.H{
			"title":   "TOTP",
			"message": "Invalid TOTP code",
		})
	}
}

func logoutHandler(c *gin.Context) {
	// Clear the session and log out the user
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/")
}

func writePageHandler(c *gin.Context) {
	// Render the "write post" page
	c.HTML(http.StatusOK, "write.html", gin.H{
		"title": "Write Post",
	})
}

func writePostHandler(c *gin.Context) {
	// Handle the form submission for writing a new post
	title := c.PostForm("title")
	content := c.PostForm("content")

	// Add the new post to the blogPosts slice
	blogPosts = append(blogPosts, BlogPost{Title: title, Content: content})

	// Redirect to the homepage
	c.Redirect(http.StatusFound, "/")
}

func authMiddleware(c *gin.Context) {
	// Check if the user is logged in
	session := sessions.Default(c)
	loggedIn := session.Get("loggedIn")
	if loggedIn == nil || !loggedIn.(bool) {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}
	c.Next()
}

func totpMiddleware(c *gin.Context) {
	// Check if the user is logged in
	session := sessions.Default(c)
	loggedIn := session.Get("loggedIn")
	if loggedIn == nil || !loggedIn.(bool) {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}
	c.Next()
}

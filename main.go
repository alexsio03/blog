package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/pquerna/otp/totp"
)

type BlogPost struct {
	ID          int64    `dynamodbav:"post"`
	DateCreated string   `dynamodbav:"date_created"`
	DateEdited  string   `dynamodbav:"date_edited"`
	Mood        string   `dynamodbav:"mood"`
	Tags        []string `dynamodbav:"tags"`
	Text        string   `dynamodbav:"text"`
	Title       string   `dynamodbav:"title"`
}

type BlogHandler struct {
	dynamoClient *dynamodb.Client
	posts        *[]BlogPost
}

type CalendarData struct {
	MonthName string
	Year      int
	PrevMonth int
	PrevYear  int
	NextMonth int
	NextYear  int
	Days      []int
  CurrentDay int
  CurrentMonth string
  CurrentYear int
}

func NewBlogHandler(client *dynamodb.Client, data *[]BlogPost) *BlogHandler {
	return &BlogHandler{
		dynamoClient: client,
		posts:        data,
	}
}

func loadConfig() ([]BlogPost, *dynamodb.Client, error) {
	if err := godotenv.Load(); err != nil {
		return nil, nil, fmt.Errorf("error loading .env file: %w", err)
	}

	// Initialize AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-west-1"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Scan DynamoDB table
	result, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName: aws.String("posts"),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan table: %w", err)
	}

	// Process results
	var posts []BlogPost
	for _, item := range result.Items {
		var post BlogPost
		if err := attributevalue.UnmarshalMap(item, &post); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal item: %w", err)
		}
		fmt.Println(post)
		posts = append(posts, post)
	}

	return posts, client, nil
}

func appendBlogPost(
	ctx context.Context,
	client *dynamodb.Client,
	posts *[]BlogPost,
	newPost BlogPost,
) error {
	// Marshal the new post into an AttributeValue map
	itemAV, err := attributevalue.MarshalMap(newPost)
	if err != nil {
		return fmt.Errorf("failed to marshal new post: %w", err)
	}

	// Use PutItem to insert a new blog post
	input := &dynamodb.PutItemInput{
		TableName: aws.String("posts"),
		Item:      itemAV,
	}

	if _, err := client.PutItem(ctx, input); err != nil {
		return fmt.Errorf("failed to insert new post into DynamoDB: %w", err)
	}
	// Update the posts slice with the new post
	*posts = append(*posts, newPost)

	return nil
}

func removePostByID(id int64, posts *[]BlogPost) []BlogPost {
	for i, post := range *posts {
		if post.ID == id {
			*posts = append((*posts)[:i], (*posts)[i+1:]...)
			break
		}
	}
	return *posts
}

func deleteBlogPost(
	ctx context.Context,
	client *dynamodb.Client,
	posts *[]BlogPost,
	id int64,
) error {
	// Delete the blog post by its numeric ID
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String("posts"),
		Key: map[string]types.AttributeValue{
			"post": &types.AttributeValueMemberN{Value: strconv.FormatInt(id, 10)},
		},
	}

	if _, err := client.DeleteItem(ctx, input); err != nil {
		return fmt.Errorf("failed to delete post from DynamoDB: %w", err)
	}
	// Remove the deleted post from the posts slice
	*posts = removePostByID(id, posts)

	return nil
}

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
	r.DELETE("/posts/:id", authMiddleware, handler.deletePostHandler)
  r.GET("/calendar", calendarHandler)

	// Start server
	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func (h *BlogHandler) homeHandler(c *gin.Context) {
	// Render the "index.html" template with blog posts
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "My Blog",
		"posts": h.posts,
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

func calendarHandler(c *gin.Context) {
		// Get month and year from query params or use current date
		month, _ := strconv.Atoi(c.DefaultQuery("month", strconv.Itoa(int(time.Now().Month()))))
		year, _ := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(time.Now().Year())))

		// Generate calendar data
		calendarData := generateCalendarData(month, year)

		// Render the calendar template
		c.HTML(http.StatusOK, "calendar.html", calendarData)
	}

func generateCalendarData(month, year int) CalendarData {
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	_, lastDay := firstDay.Month(), daysInMonth(year, month)

	// Calculate previous and next months
	prevMonth, prevYear := month-1, year
	nextMonth, nextYear := month+1, year

	if prevMonth == 0 {
		prevMonth, prevYear = 12, year-1
	}
	if nextMonth == 13 {
		nextMonth, nextYear = 1, year+1
	}

	// Generate days with placeholders for alignment
	days := make([]int, 0)
	for i := 0; i < int(firstDay.Weekday()); i++ {
		days = append(days, 0) // Empty spaces for alignment
	}
	for day := 1; day <= lastDay; day++ {
		days = append(days, day)
	}

	return CalendarData{
		MonthName: time.Month(month).String(),
		Year:      year,
		PrevMonth: prevMonth,
		PrevYear:  prevYear,
		NextMonth: nextMonth,
		NextYear:  nextYear,
		Days:      days,
    CurrentDay: time.Now().Day(),
    CurrentMonth: time.Month(time.Now().Month()).String(),
    CurrentYear: time.Now().Year(),
	}
}

// daysInMonth returns the number of days in a given month/year
func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
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

func (h *BlogHandler) deletePostHandler(c *gin.Context) {
	id := c.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid post ID")
		return
	}
	if err := deleteBlogPost(c.Request.Context(), h.dynamoClient, h.posts, idInt); err != nil {
		c.String(http.StatusInternalServerError, "Failed to delete post: %v", err)
		return
	}

	c.Header("HX-Redirect", "/")
	c.Status(http.StatusSeeOther)
}

func writePageHandler(c *gin.Context) {
	// Render the "write post" page
	c.HTML(http.StatusOK, "write.html", gin.H{
		"title": "Write Post",
	})
}

func (h *BlogHandler) writePostHandler(c *gin.Context) {
	tags := strings.Split(c.PostForm("tags"), ",")
	newPost := BlogPost{
		ID:          time.Now().Unix(),
		Title:       c.PostForm("title"),
		Text:        c.PostForm("text"),
		Tags:        tags,
		Mood:        c.PostForm("mood"),
		DateCreated: time.Now().Format("Jan 2, 2006"),
		DateEdited:  time.Now().Format("Jan 2, 2006"),
	}

	if err := appendBlogPost(c.Request.Context(), h.dynamoClient, h.posts, newPost); err != nil {
		c.String(http.StatusInternalServerError, "Failed to save post: %v", err)
		return
	}

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

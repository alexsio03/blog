package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
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
	MonthName    string
	Year         int
	PrevMonth    int
	PrevYear     int
	NextMonth    int
	NextYear     int
	Days         []int
	CurrentDay   int
	CurrentMonth string
	CurrentYear  int
}

type User struct {
	Username         string  `dynamodbav:"username"`
	LoginAttempts    int     `dynamodbav:"login_attempts"`
	LastLoginAttempt string  `dynamodbav:"last_login_attempt"`
	Timeout          float64 `dynamodbav:"timeout"`
}

func NewBlogHandler(client *dynamodb.Client, data *[]BlogPost) *BlogHandler {
	return &BlogHandler{
		dynamoClient: client,
		posts:        data,
	}
}

func (h *BlogHandler) homeHandler(c *gin.Context) {
    tags := aggregateTags(h.posts)
    sortedPosts := sortPostsByDate(*h.posts)
    
    c.HTML(http.StatusOK, "index.html", gin.H{
        "title": "My Blog",
        "posts": sortedPosts,
        "user":  sessions.Default(c).Get("loggedIn"),
        "tags":  tags,
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

func loginHandler(c *gin.Context, client *dynamodb.Client, adminUsername, adminPassword string) {
	// Simulate user authentication
	username := c.PostForm("username")
	password := c.PostForm("password")

	// Get the user item from the user table
	user, err := getUser(c, client, username)
	fmt.Println(user)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"title":   "Login",
			"message": "Failed to retrieve user",
		})
		return
	}

	if username == adminUsername && user.LoginAttempts < 3 && user.Username == adminUsername &&
		password == adminPassword {
		session := sessions.Default(c)
		session.Set("loggedIn", true)
		c.Redirect(http.StatusFound, "/totp")
	} else {
		const (
        maxAttempts = 3
        timeoutMinutes = 30
    )

    // Handle new login attempt
    if user.LoginAttempts < maxAttempts {
        user.LastLoginAttempt = time.Now().Format(time.RFC3339)
        user.LoginAttempts++
    }

    // Check if we need to start or continue timeout
    if user.LoginAttempts >= maxAttempts {
        lastAttempt, err := time.Parse(time.RFC3339, user.LastLoginAttempt)
        if err != nil {
            c.HTML(http.StatusInternalServerError, "login.html", gin.H{
                "title":   "Login",
                "message": "Failed to parse last login attempt",
            })
            return
        }

        // Calculate remaining timeout
        elapsedMinutes := time.Since(lastAttempt).Minutes()
        if elapsedMinutes < timeoutMinutes {
            user.Timeout = timeoutMinutes - elapsedMinutes
        } else {
            // Reset after timeout period
            user.LoginAttempts = 0
            user.Timeout = 0
        }
    }
		_, err := putUser(c, client, user)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "login.html", gin.H{
				"title":   "Login",
				"message": "Failed to update user",
			})
			return
		}
		attemptsLeft := 3 - user.LoginAttempts
		if attemptsLeft <= 0 {
			c.HTML(http.StatusOK, "login.html", gin.H{
				"title":   "Login",
				"message": "You have exceeded the maximum number of login attempts.",
				"timeout": math.Round(user.Timeout),
			})
			return
		} else {
			c.HTML(http.StatusOK, "login.html", gin.H{
				"title":   "Login",
				"message": "Invalid username or password",
				"logins":  attemptsLeft,
			})
			return
		}
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
	tagsRaw := strings.Split(c.PostForm("tags"), ",")
	var tags []string

	for _, tag := range tagsRaw {
		trimmedTag := strings.TrimSpace(tag) // Remove spaces around each tag
		if trimmedTag != "" {                // Ignore empty tags
			tags = append(tags, trimmedTag)
		}
	}
	newPost := BlogPost{}
	postID := time.Now().Unix()
	DateCreated := time.Now().Format("Jan 2, 2006")
	if c.PostForm("id") != "" {
		id, err := strconv.ParseInt(c.PostForm("id"), 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid post ID")
			return
		}
		post, err := getBlogPost(h.posts, id)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid post ID")
			return
		}
		postID = post.ID
		DateCreated = post.DateCreated
	}
	newPost = BlogPost{
		ID:          postID,
		Title:       c.PostForm("title"),
		Text:        c.PostForm("text"),
		Tags:        tags,
		Mood:        c.PostForm("mood"),
		DateCreated: DateCreated,
		DateEdited:  time.Now().Format("Jan 2, 2006"),
	}

	if c.PostForm("id") != "" {
		if err := updatePostByID(c.Request.Context(), h.dynamoClient, h.posts, newPost.ID, newPost); err != nil {
			c.String(http.StatusInternalServerError, "Failed to update post: %v", err)
			return
		}
	} else {
		if err := appendBlogPost(c.Request.Context(), h.dynamoClient, h.posts, newPost); err != nil {
			c.String(http.StatusInternalServerError, "Failed to save post: %v", err)
			return
		}
	}

	c.Redirect(http.StatusFound, "/")
}

func (h *BlogHandler) editPostHandler(c *gin.Context) {
	id := c.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid post ID")
		return
	}
	post, err := getBlogPost(h.posts, idInt)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get post: %v", err)
		return
	}
	c.HTML(http.StatusOK, "write.html", gin.H{
		"title": "Edit Post",
		"post":  post,
	})
}

func (h *BlogHandler) tagHandler(c *gin.Context) {
	// Render the "index.html" template with blog posts
  tags := aggregateTags(h.posts)
  posts := filterPostsByTag(h.posts, c.Param("tag"))
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "My Blog",
		"posts": posts,
		"user":  sessions.Default(c).Get("loggedIn"),
    "tags":  tags,
	})
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

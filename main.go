package main

import (
	"net/http"
  "github.com/gin-gonic/gin"
)

type BlogPost struct {
	Title   string
	Content string
}

var blogPosts = []BlogPost{
	{Title: "First Post", Content: "This is the content of the first post."},
	{Title: "Second Post", Content: "This is the content of the second post."},
	{Title: "Third Post", Content: "This is the content of the third post."},
}

func main() {
  r := gin.Default()
  r.LoadHTMLGlob("templates/*")
  r.GET("/", homeHandler)
  r.Run(":8080")
}

func homeHandler(c *gin.Context) {
  c.HTML(http.StatusOK, "index.html", gin.H{
    "title": "My Blog",
    "posts": blogPosts,
  })
}


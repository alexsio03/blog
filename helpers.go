package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/joho/godotenv"
)

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

func updatePostByID(ctx context.Context, client *dynamodb.Client, posts *[]BlogPost, id int64, updatedPost BlogPost) error {
  // Use UpdateItem to update the blog post
  tagsAttributeValues := make([]types.AttributeValue, len(updatedPost.Tags))
  for i, tag := range updatedPost.Tags {
        tagsAttributeValues[i] = &types.AttributeValueMemberS{Value: tag}
  }

  input := &dynamodb.UpdateItemInput{
        TableName: aws.String("posts"),
        Key: map[string]types.AttributeValue{
              "post": &types.AttributeValueMemberN{Value: strconv.FormatInt(id, 10)},
          },
        ExpressionAttributeNames: map[string]string{
              "#title":       "title",
              "#text":        "text",
              "#mood":        "mood",
              "#date_edited": "date_edited",
              "#date_created": "date_created",
              "#tags":        "tags", // Added #tags to the ExpressionAttributeNames map
          },
        ExpressionAttributeValues: map[string]types.AttributeValue{
              ":title":       &types.AttributeValueMemberS{Value: updatedPost.Title},
              ":text":        &types.AttributeValueMemberS{Value: updatedPost.Text},
              ":mood":        &types.AttributeValueMemberS{Value: updatedPost.Mood},
              ":date_edited": &types.AttributeValueMemberS{Value: updatedPost.DateEdited},
              ":tags":        &types.AttributeValueMemberL{Value: tagsAttributeValues}, // Use the converted list
              ":date_created": &types.AttributeValueMemberS{Value: updatedPost.DateCreated},
          },
        UpdateExpression: aws.String("SET #title = :title, #text = :text, #mood = :mood, #date_edited = :date_edited, #tags = :tags, #date_created = :date_created"),
  }

  if _, err := client.UpdateItem(ctx, input); err != nil {
    return fmt.Errorf("failed to update post in DynamoDB: %w", err)
  }

  // Update the posts slice with the updated post
  for i, post := range *posts {
    if post.ID == id {
      (*posts)[i] = updatedPost
      break
    }
  }

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

func getBlogPost(
  posts *[]BlogPost,
  id int64,
) (BlogPost, error) {
  // Get the blog post by its numeric ID from posts array
  for _, post := range *posts {
    if post.ID == id {
      return post, nil
    }
  }

  return BlogPost{}, fmt.Errorf("post not found")
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
		MonthName:    time.Month(month).String(),
		Year:         year,
		PrevMonth:    prevMonth,
		PrevYear:     prevYear,
		NextMonth:    nextMonth,
		NextYear:     nextYear,
		Days:         days,
		CurrentDay:   time.Now().Day(),
		CurrentMonth: time.Month(time.Now().Month()).String(),
		CurrentYear:  time.Now().Year(),
	}
}

// daysInMonth returns the number of days in a given month/year
func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
}

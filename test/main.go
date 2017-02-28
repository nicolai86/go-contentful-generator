package main

//go:generate go-contentful-generator -pkg main -o contentful.go

import (
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	c := New(os.Getenv("CONTENTFUL_AUTH_TOKEN"), []string{"en-US"})
	it := c.Posts(ListOptions{Limit: 1, IncludeCount: 1})
	fmt.Printf("Posts:\n")
	for {
		p, err := it.Next()
		if err == IteratorDone {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID: %s\n", p.ID)
		fmt.Printf("Title: %s, by ", p.Title)
		for _, a := range p.Author {
			fmt.Printf("%s ", a.Name)
		}
		fmt.Printf("\nCategories:")
		for _, c := range p.Category {
			fmt.Printf("%s,", c.Title)
		}
		fmt.Printf("\n")
		fmt.Printf("Tags: %v\n", p.Tags)
		fmt.Printf("Date: %s\n", time.Time(p.Date).Format("2006-01-02"))
		fmt.Printf("comments?: %v\n", p.Comments)
		fmt.Printf("\n")
	}
}

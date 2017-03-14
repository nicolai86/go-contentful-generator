package main

//go:generate go-contentful-generator -pkg main -o contentful.go

import (
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	{
		c := NewManagement(os.Getenv("CONTENTFUL_CMA_AUTH_TOKEN"))
		ws := c.Webhooks()
		// w1 := Webhook{
		// 	URL:    "https://nicolai86.eu/2",
		// 	Name:   "foo2",
		// 	Topics: []string{"*.*"},
		// }
		// if err := ws.Create(&w1); err != nil {
		// 	log.Fatalf("creation failed: %q", err.Error())
		// }
		// fmt.Printf("Created: %q\n", w1.ID)

		it := ws.List(ListOptions{Limit: 1})
		fmt.Printf("Webhooks: \n")
		for {
			w, err := it.Next()
			if err == ErrIteratorDone {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%#v (%#v)\n", w.Name, w)
			w.Name = w.Name + "2"
			w.URL = w.URL + "/3"
			if err := ws.Update(w); err != nil {
				log.Fatalf("Failed: %#v", err)
			}
			// if err := ws.Delete(w.ID); err != nil {
			//   log.Fatalf("Failed: %#v", err)
			// }
		}
	}

	{
		c := NewCDA(os.Getenv("CONTENTFUL_CDA_AUTH_TOKEN"), "en-US")
		it := c.Posts(ListOptions{Limit: 1, IncludeCount: 1})
		fmt.Printf("Posts:\n")
		for {
			p, err := it.Next()
			if err == ErrIteratorDone {
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
}

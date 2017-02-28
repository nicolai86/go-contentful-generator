package main

import (
	json "encoding/json"
	fmt "fmt"
	http "net/http"
	strings "strings"
	time "time"
)

const dateLayout = "2006-01-02"

// Date defines an ISO 8601 date only time
type Date time.Time

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		*d = Date(time.Time{})
	}
	t, err := time.Parse(dateLayout, s)
	*d = Date(t)
	return err
}

// Asset defines a media item in contentful
type Asset struct {
	Title       string
	Description string
	URL         string
	Width       int64
	Height      int64
	Size        int64
}
type includes struct {
	Entries []includeEntry `json:"Entry"`
	Assets  []includeAsset `json:"Asset"`
}
type sys struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	ContentType struct {
		Sys struct {
			ID string `json:"id"`
		} `json:"sys"`
	} `json:"contentType"`
}
type entryID struct {
	Sys sys `json:"sys"`
}
type entryIDs []entryID
type includeEntry struct {
	Sys    sys              `json:"sys"`
	Fields *json.RawMessage `json:"fields"`
}
type includeAsset struct {
	Sys    sys `json:"sys"`
	Fields struct {
		File struct {
			URL     string `json:"url"`
			Details struct {
				Image struct {
					Width  int64 `json:"width"`
					Height int64 `json:"height"`
				} `json:"image"`
			} `json:"details"`
		} `json:"file"`
	} `json:"fields"`
}

// Author a
type Author struct {
	ID             string
	Name           string
	Website        string
	ProfilePhoto   Asset
	Biography      string
	CreatedEntries []Post
	Age            int64
	Rating         float64
}

// authorItem contains a single contentful Author model
type authorItem struct {
	Sys    sys `json:"sys"`
	Fields struct {
		Name           string   `json:"name"`
		Website        string   `json:"website"`
		ProfilePhoto   entryID  `json:"profilePhoto"`
		Biography      string   `json:"biography"`
		CreatedEntries entryIDs `json:"createdEntries"`
		Age            int64    `json:"age"`
		Rating         float64  `json:"rating"`
	} `json:"fields"`
}

// authorResponse holds an entire contentful Author response
type authorResponse struct {
	Items    []authorItem `json:"items"`
	Includes includes     `json:"includes"`
}

func resolveAuthor(entryID string, includes includes) Author {
	var item authorItem
	for _, entry := range includes.Entries {
		if entry.Sys.ID == entryID {
			json.Unmarshal(*entry.Fields, &item.Fields)
			return Author{
				ID:             item.Sys.ID,
				Name:           item.Fields.Name,
				Website:        item.Fields.Website,
				ProfilePhoto:   resolveAsset(item.Fields.ProfilePhoto.Sys.ID, includes),
				Biography:      item.Fields.Biography,
				CreatedEntries: resolvePosts(item.Fields.CreatedEntries, includes),
				Age:            item.Fields.Age,
				Rating:         item.Fields.Rating,
			}
		}
	}
	return Author{}
}
func resolveAuthors(ids entryIDs, includes includes) []Author {
	var items []Author
	var item authorItem
	for _, entry := range includes.Entries {
		var included = false
		for _, entryID := range ids {
			included = included || entryID.Sys.ID == entry.Sys.ID
		}
		if included == true {
			json.Unmarshal(*entry.Fields, &item.Fields)
			items = append(items, Author{
				ID:             item.Sys.ID,
				Name:           item.Fields.Name,
				Website:        item.Fields.Website,
				ProfilePhoto:   resolveAsset(item.Fields.ProfilePhoto.Sys.ID, includes),
				Biography:      item.Fields.Biography,
				CreatedEntries: resolvePosts(item.Fields.CreatedEntries, includes),
				Age:            item.Fields.Age,
				Rating:         item.Fields.Rating,
			})
		}
	}
	return items
}

// Authors retrieves paginated Author entries
func (c *Client) Authors() ([]Author, error) {
	var url = fmt.Sprintf("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=10&locale=%s", c.host, c.spaceID, c.authToken, "1kUEViTN4EmGiEaaeC6ouY", c.Locales[0])
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	var data authorResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	resp.Body.Close()
	var items = make([]Author, len(data.Items))
	for i, item := range data.Items {
		items[i] = Author{
			ID:             item.Sys.ID,
			Name:           item.Fields.Name,
			Website:        item.Fields.Website,
			ProfilePhoto:   resolveAsset(item.Fields.ProfilePhoto.Sys.ID, data.Includes),
			Biography:      item.Fields.Biography,
			CreatedEntries: resolvePosts(item.Fields.CreatedEntries, data.Includes),
			Age:            item.Fields.Age,
			Rating:         item.Fields.Rating,
		}
	}
	return items, nil
}

// Category
type Category struct {
	ID               string
	Title            string
	ShortDescription string
	Icon             Asset
	Parent           interface{}
}

// categoryItem contains a single contentful Category model
type categoryItem struct {
	Sys    sys `json:"sys"`
	Fields struct {
		Title            string  `json:"title"`
		ShortDescription string  `json:"shortDescription"`
		Icon             entryID `json:"icon"`
		Parent           entryID `json:"parent"`
	} `json:"fields"`
}

// categoryResponse holds an entire contentful Category response
type categoryResponse struct {
	Items    []categoryItem `json:"items"`
	Includes includes       `json:"includes"`
}

func resolveCategory(entryID string, includes includes) Category {
	var item categoryItem
	for _, entry := range includes.Entries {
		if entry.Sys.ID == entryID {
			json.Unmarshal(*entry.Fields, &item.Fields)
			return Category{
				ID:               item.Sys.ID,
				Title:            item.Fields.Title,
				ShortDescription: item.Fields.ShortDescription,
				Icon:             resolveAsset(item.Fields.Icon.Sys.ID, includes),
				Parent:           resolveEntry(item.Fields.Parent, includes),
			}
		}
	}
	return Category{}
}
func resolveCategorys(ids entryIDs, includes includes) []Category {
	var items []Category
	var item categoryItem
	for _, entry := range includes.Entries {
		var included = false
		for _, entryID := range ids {
			included = included || entryID.Sys.ID == entry.Sys.ID
		}
		if included == true {
			json.Unmarshal(*entry.Fields, &item.Fields)
			items = append(items, Category{
				ID:               item.Sys.ID,
				Title:            item.Fields.Title,
				ShortDescription: item.Fields.ShortDescription,
				Icon:             resolveAsset(item.Fields.Icon.Sys.ID, includes),
				Parent:           resolveEntry(item.Fields.Parent, includes),
			})
		}
	}
	return items
}

// Categories retrieves paginated Category entries
func (c *Client) Categories() ([]Category, error) {
	var url = fmt.Sprintf("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=10&locale=%s", c.host, c.spaceID, c.authToken, "5KMiN6YPvi42icqAUQMCQe", c.Locales[0])
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	var data categoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	resp.Body.Close()
	var items = make([]Category, len(data.Items))
	for i, item := range data.Items {
		items[i] = Category{
			ID:               item.Sys.ID,
			Title:            item.Fields.Title,
			ShortDescription: item.Fields.ShortDescription,
			Icon:             resolveAsset(item.Fields.Icon.Sys.ID, data.Includes),
			Parent:           resolveEntry(item.Fields.Parent, data.Includes),
		}
	}
	return items, nil
}

// Post
type Post struct {
	ID            string
	Title         string
	Slug          string
	Author        []Author
	Body          string
	Category      []Category
	Tags          []string
	FeaturedImage Asset
	Date          Date
	Comments      bool
	Approver      interface{}
	AuthorOrPost  []interface{}
}

// postItem contains a single contentful Post model
type postItem struct {
	Sys    sys `json:"sys"`
	Fields struct {
		Title         string   `json:"title"`
		Slug          string   `json:"slug"`
		Author        entryIDs `json:"author"`
		Body          string   `json:"body"`
		Category      entryIDs `json:"category"`
		Tags          []string `json:"tags"`
		FeaturedImage entryID  `json:"featuredImage"`
		Date          Date     `json:"date"`
		Comments      bool     `json:"comments"`
		Approver      entryID  `json:"approver"`
		AuthorOrPost  entryIDs `json:"authorOrPost"`
	} `json:"fields"`
}

// postResponse holds an entire contentful Post response
type postResponse struct {
	Items    []postItem `json:"items"`
	Includes includes   `json:"includes"`
}

func resolvePost(entryID string, includes includes) Post {
	var item postItem
	for _, entry := range includes.Entries {
		if entry.Sys.ID == entryID {
			json.Unmarshal(*entry.Fields, &item.Fields)
			return Post{
				ID:            item.Sys.ID,
				Title:         item.Fields.Title,
				Slug:          item.Fields.Slug,
				Author:        resolveAuthors(item.Fields.Author, includes),
				Body:          item.Fields.Body,
				Category:      resolveCategorys(item.Fields.Category, includes),
				Tags:          item.Fields.Tags,
				FeaturedImage: resolveAsset(item.Fields.FeaturedImage.Sys.ID, includes),
				Date:          item.Fields.Date,
				Comments:      item.Fields.Comments,
				Approver:      resolveEntry(item.Fields.Approver, includes),
				AuthorOrPost:  resolveEntries(item.Fields.AuthorOrPost, includes),
			}
		}
	}
	return Post{}
}
func resolvePosts(ids entryIDs, includes includes) []Post {
	var items []Post
	var item postItem
	for _, entry := range includes.Entries {
		var included = false
		for _, entryID := range ids {
			included = included || entryID.Sys.ID == entry.Sys.ID
		}
		if included == true {
			json.Unmarshal(*entry.Fields, &item.Fields)
			items = append(items, Post{
				ID:            item.Sys.ID,
				Title:         item.Fields.Title,
				Slug:          item.Fields.Slug,
				Author:        resolveAuthors(item.Fields.Author, includes),
				Body:          item.Fields.Body,
				Category:      resolveCategorys(item.Fields.Category, includes),
				Tags:          item.Fields.Tags,
				FeaturedImage: resolveAsset(item.Fields.FeaturedImage.Sys.ID, includes),
				Date:          item.Fields.Date,
				Comments:      item.Fields.Comments,
				Approver:      resolveEntry(item.Fields.Approver, includes),
				AuthorOrPost:  resolveEntries(item.Fields.AuthorOrPost, includes),
			})
		}
	}
	return items
}

// Posts retrieves paginated Post entries
func (c *Client) Posts() ([]Post, error) {
	var url = fmt.Sprintf("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=10&locale=%s", c.host, c.spaceID, c.authToken, "2wKn6yEnZewu2SCCkus4as", c.Locales[0])
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	var data postResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	resp.Body.Close()
	var items = make([]Post, len(data.Items))
	for i, item := range data.Items {
		items[i] = Post{
			ID:            item.Sys.ID,
			Title:         item.Fields.Title,
			Slug:          item.Fields.Slug,
			Author:        resolveAuthors(item.Fields.Author, data.Includes),
			Body:          item.Fields.Body,
			Category:      resolveCategorys(item.Fields.Category, data.Includes),
			Tags:          item.Fields.Tags,
			FeaturedImage: resolveAsset(item.Fields.FeaturedImage.Sys.ID, data.Includes),
			Date:          item.Fields.Date,
			Comments:      item.Fields.Comments,
			Approver:      resolveEntry(item.Fields.Approver, data.Includes),
			AuthorOrPost:  resolveEntries(item.Fields.AuthorOrPost, data.Includes),
		}
	}
	return items, nil
}
func resolveAsset(assetID string, includes includes) Asset {
	for _, asset := range includes.Assets {
		if asset.Sys.ID == assetID {
			return Asset{
				URL:    fmt.Sprintf("https:%s", asset.Fields.File.URL),
				Width:  asset.Fields.File.Details.Image.Width,
				Height: asset.Fields.File.Details.Image.Height,
				Size:   0,
			}
		}
	}
	return Asset{}
}
func resolveEntries(ids entryIDs, includes includes) []interface{} {
	var items []interface{}
	for _, entry := range includes.Entries {
		var included = false
		for _, entryID := range ids {
			included = included || entryID.Sys.ID == entry.Sys.ID
		}
		if included == true {
			if entry.Sys.ContentType.Sys.ID == "1kUEViTN4EmGiEaaeC6ouY" {
				items = append(items, resolveAuthor(entry.Sys.ID, includes))
			}
			if entry.Sys.ContentType.Sys.ID == "5KMiN6YPvi42icqAUQMCQe" {
				items = append(items, resolveCategory(entry.Sys.ID, includes))
			}
			if entry.Sys.ContentType.Sys.ID == "2wKn6yEnZewu2SCCkus4as" {
				items = append(items, resolvePost(entry.Sys.ID, includes))
			}
		}
	}
	return items
}
func resolveEntry(id entryID, includes includes) interface{} {
	for _, entry := range includes.Entries {
		if entry.Sys.ID == id.Sys.ID {
			if entry.Sys.ContentType.Sys.ID == "1kUEViTN4EmGiEaaeC6ouY" {
				return resolveAuthor(entry.Sys.ID, includes)
			}
			if entry.Sys.ContentType.Sys.ID == "5KMiN6YPvi42icqAUQMCQe" {
				return resolveCategory(entry.Sys.ID, includes)
			}
			if entry.Sys.ContentType.Sys.ID == "2wKn6yEnZewu2SCCkus4as" {
				return resolvePost(entry.Sys.ID, includes)
			}
		}
	}
	return nil
}

// Client
type Client struct {
	host      string
	spaceID   string
	authToken string
	Locales   []string
	client    *http.Client
}

// New returns a contentful client
func New(authToken string, locales []string) *Client {
	return &Client{
		host:      "https://cdn.contentful.com",
		spaceID:   "ygx37epqlss8",
		authToken: authToken,
		Locales:   locales,
		client:    &http.Client{},
	}
}

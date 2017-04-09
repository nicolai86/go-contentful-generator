package main

import (
	bytes "bytes"
	tls "crypto/tls"
	x509 "crypto/x509"
	json "encoding/json"
	fmt "fmt"
	http "net/http"
	strings "strings"
	time "time"
)

const dateLayout = "2006-01-02"

// Date defines an ISO 8601 date only time
type Date time.Time

// UnmarshalJSON deserializes an iso 8601 short date string
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
	Version     int    `json:"version"`
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
type iteratorCache struct {
	posts     map[string]*Post
	authors   map[string]*Author
	categorys map[string]*Category
}

// PostIterator is used to paginate result sets of Post
type PostIterator struct {
	Page         int
	Limit        int
	Offset       int
	IncludeCount int
	c            *ContentClient
	items        []*Post
	lookupCache  *iteratorCache
}

// Next returns the following item of type Post. If none exists a network request will be executed
func (it *PostIterator) Next() (*Post, error) {
	if len(it.items) == 0 {
		if err := it.fetch(); err != nil {
			return nil, err
		}
	}
	if len(it.items) == 0 {
		return nil, ErrIteratorDone
	}
	var item *Post
	item, it.items = it.items[len(it.items)-1], it.items[:len(it.items)-1]
	if len(it.items) == 0 {
		it.Page++
		it.Offset = it.Page * it.Limit
	}
	return item, nil
}
func (it *PostIterator) fetch() error {
	c := it.c
	var url = fmt.Sprintf("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=%d&locale=%s&limit=%d&skip=%d", c.host, c.spaceID, c.authToken, "2wKn6yEnZewu2SCCkus4as", it.IncludeCount, c.Locale, it.Limit, it.Offset)
	resp, err := c.client.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	var data postResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	var items = make([]*Post, len(data.Items))
	for i, raw := range data.Items {
		var item postItem
		if err := json.Unmarshal(*raw.Fields, &item.Fields); err != nil {
			return err
		}
		items[i] = &Post{
			Approver:      resolveAuthor(item.Fields.Approver.Sys.ID, data.Items, data.Includes, it.lookupCache),
			Author:        resolveAuthors(item.Fields.Author, data.Items, data.Includes, it.lookupCache),
			AuthorOrPost:  resolveEntries(item.Fields.AuthorOrPost, data.Items, data.Includes, it.lookupCache),
			Body:          item.Fields.Body,
			Category:      resolveCategorys(item.Fields.Category, data.Items, data.Includes, it.lookupCache),
			Comments:      item.Fields.Comments,
			Date:          item.Fields.Date,
			FeaturedImage: resolveAsset(item.Fields.FeaturedImage.Sys.ID, data.Includes),
			ID:            raw.Sys.ID,
			Slug:          item.Fields.Slug,
			Title:         item.Fields.Title,
		}
	}
	it.items = items
	return nil
}

// Post has no description in contentful
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
	Approver      Author
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
	Total    int            `json:"total"`
	Skip     int            `json:"skip"`
	Limit    int            `json:"limit"`
	Items    []includeEntry `json:"items"`
	Includes includes       `json:"includes"`
}

func resolvePost(entryID string, items []includeEntry, includes includes, cache *iteratorCache) Post {
	if v, ok := cache.posts[entryID]; ok {
		return *v
	}
	var item postItem
	for _, entry := range append(includes.Entries, items...) {
		if entry.Sys.ID == entryID {
			if err := json.Unmarshal(*entry.Fields, &item.Fields); err != nil {
				return Post{}
			}
			var tmp = &Post{
				Body:     item.Fields.Body,
				Comments: item.Fields.Comments,
				Date:     item.Fields.Date,
				ID:       entry.Sys.ID,
				Slug:     item.Fields.Slug,
				Title:    item.Fields.Title,
			}
			cache.posts[entry.Sys.ID] = tmp
			tmp.Approver = resolveAuthor(item.Fields.Approver.Sys.ID, items, includes, cache)
			tmp.AuthorOrPost = resolveEntries(item.Fields.AuthorOrPost, items, includes, cache)
			tmp.Author = resolveAuthors(item.Fields.Author, items, includes, cache)
			tmp.Category = resolveCategorys(item.Fields.Category, items, includes, cache)
			tmp.FeaturedImage = resolveAsset(item.Fields.FeaturedImage.Sys.ID, includes)
			return *tmp
		}
	}
	return Post{}
}
func resolvePostPtr(entryID string, items []includeEntry, includes includes, cache *iteratorCache) *Post {
	var item = resolvePost(entryID, items, includes, cache)
	return &item
}
func resolvePostsPtr(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []*Post {
	var items = resolvePosts(ids, its, includes, cache)
	var ptrs []*Post
	for i := range items {
		ptrs = append(ptrs, &items[i])
	}
	return ptrs
}
func resolvePosts(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []Post {
	var items []Post
	entries := append(includes.Entries, its...)
	for _, entryID := range ids {
		var item postItem
		var entry *includeEntry
		for _, e := range entries {
			if e.Sys.ID == entryID.Sys.ID {
				entry = &e
				break
			}
		}
		if entry == nil {
			continue
		}
		if v, ok := cache.posts[entry.Sys.ID]; ok {
			items = append(items, *v)
			continue
		}
		if err := json.Unmarshal(*entry.Fields, &item.Fields); err != nil {
			return items
		}
		var tmp = &Post{
			Body:     item.Fields.Body,
			Comments: item.Fields.Comments,
			Date:     item.Fields.Date,
			ID:       entry.Sys.ID,
			Slug:     item.Fields.Slug,
			Title:    item.Fields.Title,
		}
		cache.posts[entry.Sys.ID] = tmp
		tmp.Author = resolveAuthors(item.Fields.Author, its, includes, cache)
		tmp.Category = resolveCategorys(item.Fields.Category, its, includes, cache)
		tmp.FeaturedImage = resolveAsset(item.Fields.FeaturedImage.Sys.ID, includes)
		tmp.Approver = resolveAuthor(item.Fields.Approver.Sys.ID, its, includes, cache)
		tmp.AuthorOrPost = resolveEntries(item.Fields.AuthorOrPost, its, includes, cache)
		items = append(items, *tmp)
	}
	return items
}

// Posts retrieves paginated Post entries
func (c *ContentClient) Posts(opts ListOptions) *PostIterator {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	it := &PostIterator{
		IncludeCount: opts.IncludeCount,
		Limit:        opts.Limit,
		Page:         opts.Page,
		c:            c,
		lookupCache: &iteratorCache{
			authors:   make(map[string]*Author),
			categorys: make(map[string]*Category),
			posts:     make(map[string]*Post),
		},
	}
	return it
}

// AuthorIterator is used to paginate result sets of Author
type AuthorIterator struct {
	Page         int
	Limit        int
	Offset       int
	IncludeCount int
	c            *ContentClient
	items        []*Author
	lookupCache  *iteratorCache
}

// Next returns the following item of type Author. If none exists a network request will be executed
func (it *AuthorIterator) Next() (*Author, error) {
	if len(it.items) == 0 {
		if err := it.fetch(); err != nil {
			return nil, err
		}
	}
	if len(it.items) == 0 {
		return nil, ErrIteratorDone
	}
	var item *Author
	item, it.items = it.items[len(it.items)-1], it.items[:len(it.items)-1]
	if len(it.items) == 0 {
		it.Page++
		it.Offset = it.Page * it.Limit
	}
	return item, nil
}
func (it *AuthorIterator) fetch() error {
	c := it.c
	var url = fmt.Sprintf("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=%d&locale=%s&limit=%d&skip=%d", c.host, c.spaceID, c.authToken, "1kUEViTN4EmGiEaaeC6ouY", it.IncludeCount, c.Locale, it.Limit, it.Offset)
	resp, err := c.client.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	var data authorResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	var items = make([]*Author, len(data.Items))
	for i, raw := range data.Items {
		var item authorItem
		if err := json.Unmarshal(*raw.Fields, &item.Fields); err != nil {
			return err
		}
		items[i] = &Author{
			Age:            item.Fields.Age,
			Biography:      item.Fields.Biography,
			CreatedEntries: resolvePosts(item.Fields.CreatedEntries, data.Items, data.Includes, it.lookupCache),
			ID:             raw.Sys.ID,
			Name:           item.Fields.Name,
			ProfilePhoto:   resolveAsset(item.Fields.ProfilePhoto.Sys.ID, data.Includes),
			Rating:         item.Fields.Rating,
			Website:        item.Fields.Website,
		}
	}
	it.items = items
	return nil
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
	Total    int            `json:"total"`
	Skip     int            `json:"skip"`
	Limit    int            `json:"limit"`
	Items    []includeEntry `json:"items"`
	Includes includes       `json:"includes"`
}

func resolveAuthor(entryID string, items []includeEntry, includes includes, cache *iteratorCache) Author {
	if v, ok := cache.authors[entryID]; ok {
		return *v
	}
	var item authorItem
	for _, entry := range append(includes.Entries, items...) {
		if entry.Sys.ID == entryID {
			if err := json.Unmarshal(*entry.Fields, &item.Fields); err != nil {
				return Author{}
			}
			var tmp = &Author{
				Age:       item.Fields.Age,
				Biography: item.Fields.Biography,
				ID:        entry.Sys.ID,
				Name:      item.Fields.Name,
				Rating:    item.Fields.Rating,
				Website:   item.Fields.Website,
			}
			cache.authors[entry.Sys.ID] = tmp
			tmp.ProfilePhoto = resolveAsset(item.Fields.ProfilePhoto.Sys.ID, includes)
			tmp.CreatedEntries = resolvePosts(item.Fields.CreatedEntries, items, includes, cache)
			return *tmp
		}
	}
	return Author{}
}
func resolveAuthorPtr(entryID string, items []includeEntry, includes includes, cache *iteratorCache) *Author {
	var item = resolveAuthor(entryID, items, includes, cache)
	return &item
}
func resolveAuthorsPtr(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []*Author {
	var items = resolveAuthors(ids, its, includes, cache)
	var ptrs []*Author
	for i := range items {
		ptrs = append(ptrs, &items[i])
	}
	return ptrs
}
func resolveAuthors(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []Author {
	var items []Author
	entries := append(includes.Entries, its...)
	for _, entryID := range ids {
		var item authorItem
		var entry *includeEntry
		for _, e := range entries {
			if e.Sys.ID == entryID.Sys.ID {
				entry = &e
				break
			}
		}
		if entry == nil {
			continue
		}
		if v, ok := cache.authors[entry.Sys.ID]; ok {
			items = append(items, *v)
			continue
		}
		if err := json.Unmarshal(*entry.Fields, &item.Fields); err != nil {
			return items
		}
		var tmp = &Author{
			Age:       item.Fields.Age,
			Biography: item.Fields.Biography,
			ID:        entry.Sys.ID,
			Name:      item.Fields.Name,
			Rating:    item.Fields.Rating,
			Website:   item.Fields.Website,
		}
		cache.authors[entry.Sys.ID] = tmp
		tmp.ProfilePhoto = resolveAsset(item.Fields.ProfilePhoto.Sys.ID, includes)
		tmp.CreatedEntries = resolvePosts(item.Fields.CreatedEntries, its, includes, cache)
		items = append(items, *tmp)
	}
	return items
}

// Authors retrieves paginated Author entries
func (c *ContentClient) Authors(opts ListOptions) *AuthorIterator {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	it := &AuthorIterator{
		IncludeCount: opts.IncludeCount,
		Limit:        opts.Limit,
		Page:         opts.Page,
		c:            c,
		lookupCache: &iteratorCache{
			authors:   make(map[string]*Author),
			categorys: make(map[string]*Category),
			posts:     make(map[string]*Post),
		},
	}
	return it
}

// CategoryIterator is used to paginate result sets of Category
type CategoryIterator struct {
	Page         int
	Limit        int
	Offset       int
	IncludeCount int
	c            *ContentClient
	items        []*Category
	lookupCache  *iteratorCache
}

// Next returns the following item of type Category. If none exists a network request will be executed
func (it *CategoryIterator) Next() (*Category, error) {
	if len(it.items) == 0 {
		if err := it.fetch(); err != nil {
			return nil, err
		}
	}
	if len(it.items) == 0 {
		return nil, ErrIteratorDone
	}
	var item *Category
	item, it.items = it.items[len(it.items)-1], it.items[:len(it.items)-1]
	if len(it.items) == 0 {
		it.Page++
		it.Offset = it.Page * it.Limit
	}
	return item, nil
}
func (it *CategoryIterator) fetch() error {
	c := it.c
	var url = fmt.Sprintf("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=%d&locale=%s&limit=%d&skip=%d", c.host, c.spaceID, c.authToken, "5KMiN6YPvi42icqAUQMCQe", it.IncludeCount, c.Locale, it.Limit, it.Offset)
	resp, err := c.client.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	var data categoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	var items = make([]*Category, len(data.Items))
	for i, raw := range data.Items {
		var item categoryItem
		if err := json.Unmarshal(*raw.Fields, &item.Fields); err != nil {
			return err
		}
		items[i] = &Category{
			ID:               raw.Sys.ID,
			Icon:             resolveAsset(item.Fields.Icon.Sys.ID, data.Includes),
			Parent:           resolveCategoryPtr(item.Fields.Parent.Sys.ID, data.Items, data.Includes, it.lookupCache),
			ShortDescription: item.Fields.ShortDescription,
			Title:            item.Fields.Title,
		}
	}
	it.items = items
	return nil
}

// Category has no description in contentful
type Category struct {
	ID               string
	Title            string
	ShortDescription string
	Icon             Asset
	Parent           *Category
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
	Total    int            `json:"total"`
	Skip     int            `json:"skip"`
	Limit    int            `json:"limit"`
	Items    []includeEntry `json:"items"`
	Includes includes       `json:"includes"`
}

func resolveCategory(entryID string, items []includeEntry, includes includes, cache *iteratorCache) Category {
	if v, ok := cache.categorys[entryID]; ok {
		return *v
	}
	var item categoryItem
	for _, entry := range append(includes.Entries, items...) {
		if entry.Sys.ID == entryID {
			if err := json.Unmarshal(*entry.Fields, &item.Fields); err != nil {
				return Category{}
			}
			var tmp = &Category{
				ID:               entry.Sys.ID,
				ShortDescription: item.Fields.ShortDescription,
				Title:            item.Fields.Title,
			}
			cache.categorys[entry.Sys.ID] = tmp
			tmp.Icon = resolveAsset(item.Fields.Icon.Sys.ID, includes)
			tmp.Parent = resolveCategoryPtr(item.Fields.Parent.Sys.ID, items, includes, cache)
			return *tmp
		}
	}
	return Category{}
}
func resolveCategoryPtr(entryID string, items []includeEntry, includes includes, cache *iteratorCache) *Category {
	var item = resolveCategory(entryID, items, includes, cache)
	return &item
}
func resolveCategorysPtr(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []*Category {
	var items = resolveCategorys(ids, its, includes, cache)
	var ptrs []*Category
	for i := range items {
		ptrs = append(ptrs, &items[i])
	}
	return ptrs
}
func resolveCategorys(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []Category {
	var items []Category
	entries := append(includes.Entries, its...)
	for _, entryID := range ids {
		var item categoryItem
		var entry *includeEntry
		for _, e := range entries {
			if e.Sys.ID == entryID.Sys.ID {
				entry = &e
				break
			}
		}
		if entry == nil {
			continue
		}
		if v, ok := cache.categorys[entry.Sys.ID]; ok {
			items = append(items, *v)
			continue
		}
		if err := json.Unmarshal(*entry.Fields, &item.Fields); err != nil {
			return items
		}
		var tmp = &Category{
			ID:               entry.Sys.ID,
			ShortDescription: item.Fields.ShortDescription,
			Title:            item.Fields.Title,
		}
		cache.categorys[entry.Sys.ID] = tmp
		tmp.Icon = resolveAsset(item.Fields.Icon.Sys.ID, includes)
		tmp.Parent = resolveCategoryPtr(item.Fields.Parent.Sys.ID, its, includes, cache)
		items = append(items, *tmp)
	}
	return items
}

// Categories retrieves paginated Category entries
func (c *ContentClient) Categories(opts ListOptions) *CategoryIterator {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	it := &CategoryIterator{
		IncludeCount: opts.IncludeCount,
		Limit:        opts.Limit,
		Page:         opts.Page,
		c:            c,
		lookupCache: &iteratorCache{
			authors:   make(map[string]*Author),
			categorys: make(map[string]*Category),
			posts:     make(map[string]*Post),
		},
	}
	return it
}

// ErrIteratorDone is used to indicate that the iterator has no more data
var ErrIteratorDone = fmt.Errorf("IteratorDone")

// ListOptions contains pagination configuration for iterators
type ListOptions struct {
	Page         int
	Limit        int
	IncludeCount int
}

func resolveAsset(assetID string, includes includes) Asset {
	for _, asset := range includes.Assets {
		if asset.Sys.ID == assetID {
			return Asset{
				Height: asset.Fields.File.Details.Image.Height,
				Size:   0,
				URL:    fmt.Sprintf("https:%s", asset.Fields.File.URL),
				Width:  asset.Fields.File.Details.Image.Width,
			}
		}
	}
	return Asset{}
}
func resolveEntries(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []interface{} {
	var items []interface{}
	for _, entry := range includes.Entries {
		var included = false
		for _, entryID := range ids {
			included = included || entryID.Sys.ID == entry.Sys.ID
		}
		if included == true {
			if entry.Sys.ContentType.Sys.ID == "2wKn6yEnZewu2SCCkus4as" {
				items = append(items, resolvePost(entry.Sys.ID, its, includes, cache))
			}
			if entry.Sys.ContentType.Sys.ID == "1kUEViTN4EmGiEaaeC6ouY" {
				items = append(items, resolveAuthor(entry.Sys.ID, its, includes, cache))
			}
			if entry.Sys.ContentType.Sys.ID == "5KMiN6YPvi42icqAUQMCQe" {
				items = append(items, resolveCategory(entry.Sys.ID, its, includes, cache))
			}
		}
	}
	return items
}
func resolveEntry(id entryID, its []includeEntry, includes includes, cache *iteratorCache) interface{} {
	for _, entry := range includes.Entries {
		if entry.Sys.ID == id.Sys.ID {
			if entry.Sys.ContentType.Sys.ID == "2wKn6yEnZewu2SCCkus4as" {
				return resolvePost(entry.Sys.ID, its, includes, cache)
			}
			if entry.Sys.ContentType.Sys.ID == "1kUEViTN4EmGiEaaeC6ouY" {
				return resolveAuthor(entry.Sys.ID, its, includes, cache)
			}
			if entry.Sys.ContentType.Sys.ID == "5KMiN6YPvi42icqAUQMCQe" {
				return resolveCategory(entry.Sys.ID, its, includes, cache)
			}
		}
	}
	return nil
}

// ContentClient implements a space specific contentful client
type ContentClient struct {
	host      string
	spaceID   string
	authToken string
	Locale    string
	client    *http.Client
	pool      *x509.CertPool
}

// contentfulCDAURL points to the contentful delivery api endpoint
const contentfulCDAURL = "cdn.contentful.com"

// contentfulCDAURL points to the contentful preview api endpoint
const contentfulCPAURL = "preview.contentful.com"

// contentfulCDAURL points to the contentful management api endpoint
const contentfulCMAURL = "api.contentful.com"

// NewCDA returns a contentful client interfacing with the content delivery api
func NewCDA(authToken string, locale string) *ContentClient {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte("-----BEGIN CERTIFICATE-----\nMIIL6TCCCtGgAwIBAgIQBigdNnW0H8yz/xj67Pj93zANBgkqhkiG9w0BAQsFADBw\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3\nd3cuZGlnaWNlcnQuY29tMS8wLQYDVQQDEyZEaWdpQ2VydCBTSEEyIEhpZ2ggQXNz\ndXJhbmNlIFNlcnZlciBDQTAeFw0xNDEyMDgwMDAwMDBaFw0xODAyMDYxMjAwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRUwEwYDVQQKEwxGYXN0bHksIEluYy4xGTAXBgNVBAMTEGEu\nc3NsLmZhc3RseS5uZXQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDU\nJUiQsaVP/vC4Mb3aJUmA9KnMQa7EJfjYLsE4F0VehrOp8jlSSXmQLELlUAwPp2F2\nPNyB32DDOFBHZIYwFrApFEzsJdTKQUYk6xHPZOdYoIijpmfb5xRMdTjqxThGkk+k\nhU0+ipPWiErJNRkapLgPwPD4ctd5X8rnKF8lMHIxx5Xhdg6PqZC3F7y45Nym2a3M\n8xIKIkB77o1bkuDpGnV9ZESC/Yf9Mc4NmWrQjqQc+8yIabir+n7/YcM5UdUjZPNS\nhgL4jLYVJ+KDRZcjIT/dXRZoPpJgRFL9NIep/eSAzQa3g659uW7tjN6tg5iQm4hw\nksaWp+zfTAJc4IXNtlndAgMBAAGjggiBMIIIfTAfBgNVHSMEGDAWgBRRaP+QrwIH\ndTzM2WVkYqISuFlyOzAdBgNVHQ4EFgQUwIj0Y03ka1Q28RLCtKWy4nN7FIgwggax\nBgNVHREEggaoMIIGpIIQYS5zc2wuZmFzdGx5Lm5ldIISKi5hLnNzbC5mYXN0bHku\nbmV0gg9mYXN0Lndpc3RpYS5jb22CEHB1cmdlLmZhc3RseS5uZXSCEm1pcnJvcnMu\nZmFzdGx5Lm5ldIIOKi5wYXJzZWNkbi5jb22CDSouZmFzdHNzbC5uZXSCCXZveGVy\nLmNvbYINd3d3LnZveGVyLmNvbYIOKi5maXJlYmFzZS5jb22CEHNpdGVzLnlhbW1l\nci5jb22CGHNpdGVzLnN0YWdpbmcueWFtbWVyLmNvbYIPKi5za2ltbGlua3MuY29t\nghMqLnNraW1yZXNvdXJjZXMuY29tghBjZG4udGhpbmdsaW5rLm1lggwqLmZpdGJp\ndC5jb22CEiouaG9zdHMuZmFzdGx5Lm5ldIISY29udHJvbC5mYXN0bHkubmV0gg8q\nLndpa2lhLWluYy5jb22CFSoucGVyZmVjdGF1ZGllbmNlLmNvbYILKi53aWtpYS5j\nb22CEmYuY2xvdWQuZ2l0aHViLmNvbYIVKi5kaWdpdGFsc2Npcm9jY28ubmV0ggoq\nLmV0c3kuY29tghAqLmV0c3lzdGF0aWMuY29tgg0qLmFkZHRoaXMuY29tghAqLmFk\nZHRoaXNjZG4uY29tgg9mYXN0Lndpc3RpYS5uZXSCDnJhdy5naXRodWIuY29tgg93\nd3cudXNlcmZveC5jb22CEyouYXNzZXRzLXlhbW1lci5jb22CGyouc3RhZ2luZy5h\nc3NldHMteWFtbWVyLmNvbYIWYXNzZXRzLmh1Z2dpZXMtY2RuLm5ldIISb3JiaXQu\nc2hhemFtaWQuY29tgg9hYm91dC5qc3Rvci5vcmeCFyouZ2xvYmFsLnNzbC5mYXN0\nbHkubmV0gg13ZWIudm94ZXIuY29tgg9weXBpLnB5dGhvbi5vcmeCCyouMTJ3YnQu\nY29tghJ3d3cuaG9sZGVyZGVvcmQubm+CGnNlY3VyZWQuaW5kbi5pbmZvbGlua3Mu\nY29tghBwbGF5LnZpZHlhcmQuY29tghhwbGF5LXN0YWdpbmcudmlkeWFyZC5jb22C\nFXNlY3VyZS5pbWcud2ZyY2RuLmNvbYIWc2VjdXJlLmltZy5qb3NzY2RuLmNvbYIQ\nKi5nb2NhcmRsZXNzLmNvbYIVd2lkZ2V0cy5waW50ZXJlc3QuY29tgg4qLjdkaWdp\ndGFsLmNvbYINKi43c3RhdGljLmNvbYIPcC5kYXRhZG9naHEuY29tghBuZXcubXVs\nYmVycnkuY29tghJ3d3cuc2FmYXJpZmxvdy5jb22CEmNkbi5jb250ZW50ZnVsLmNv\nbYIQdG9vbHMuZmFzdGx5Lm5ldIISKi5odWV2b3NidWVub3MuY29tgg4qLmdvb2Rl\nZ2dzLmNvbYIWKi5mYXN0bHkucGljbW9ua2V5LmNvbYIVKi5jZG4ud2hpcHBsZWhp\nbGwubmV0ghEqLndoaXBwbGVoaWxsLm5ldIIbY2RuLm1lZGlhMzQud2hpcHBsZWhp\nbGwubmV0ghtjZG4ubWVkaWE1Ni53aGlwcGxlaGlsbC5uZXSCG2Nkbi5tZWRpYTc4\nLndoaXBwbGVoaWxsLm5ldIIcY2RuLm1lZGlhOTEwLndoaXBwbGVoaWxsLm5ldIIO\nKi5tb2RjbG90aC5jb22CDyouZGlzcXVzY2RuLmNvbYILKi5qc3Rvci5vcmeCDyou\nZHJlYW1ob3N0LmNvbYIOd3d3LmZsaW50by5jb22CDyouY2hhcnRiZWF0LmNvbYIN\nKi5oaXBtdW5rLmNvbYIaY29udGVudC5iZWF2ZXJicm9va3MuY28udWuCG3NlY3Vy\nZS5jb21tb24uY3Nuc3RvcmVzLmNvbYIOd3d3LmpvaW5vcy5jb22CJXN0YWdpbmct\nbW9iaWxlLWNvbGxlY3Rvci5uZXdyZWxpYy5jb22CDioubW9kY2xvdGgubmV0ghAq\nLmZvdXJzcXVhcmUuY29tggwqLnNoYXphbS5jb22CCiouNHNxaS5uZXSCDioubWV0\nYWNwYW4ub3JnggwqLmZhc3RseS5jb22CCXdpa2lhLmNvbYIKZmFzdGx5LmNvbYIR\nKi5nYWR2ZW50dXJlcy5jb22CFnd3dy5nYWR2ZW50dXJlcy5jb20uYXWCFXd3dy5n\nYWR2ZW50dXJlcy5jby51a4IJa3JlZG8uY29tghZjZG4tdGFncy5icmFpbmllbnQu\nY29tghRteS5iaWxsc3ByaW5nYXBwLmNvbYIGcnZtLmlvMA4GA1UdDwEB/wQEAwIF\noDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwdQYDVR0fBG4wbDA0oDKg\nMIYuaHR0cDovL2NybDMuZGlnaWNlcnQuY29tL3NoYTItaGEtc2VydmVyLWc1LmNy\nbDA0oDKgMIYuaHR0cDovL2NybDQuZGlnaWNlcnQuY29tL3NoYTItaGEtc2VydmVy\nLWc1LmNybDBMBgNVHSAERTBDMDcGCWCGSAGG/WwBATAqMCgGCCsGAQUFBwIBFhxo\ndHRwczovL3d3dy5kaWdpY2VydC5jb20vQ1BTMAgGBmeBDAECAjCBgwYIKwYBBQUH\nAQEEdzB1MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2VydC5jb20wTQYI\nKwYBBQUHMAKGQWh0dHA6Ly9jYWNlcnRzLmRpZ2ljZXJ0LmNvbS9EaWdpQ2VydFNI\nQTJIaWdoQXNzdXJhbmNlU2VydmVyQ0EuY3J0MAwGA1UdEwEB/wQCMAAwDQYJKoZI\nhvcNAQELBQADggEBAKLWzbX7wSyjzE7BVMjLrHAaiz+WGSwrAPrQBJ29sqouu9gv\nI7i2Ie6eiRb4YLMouy6D+ZNZ+RM+Hkjv+PZFxCcDRmaWi+74ha5d8O155gRJRPZ0\nSy5SfD/8kqrJRfC+/D/KdQzOroD4sx6Qprs9lZ0IEn4CTf0YPNV+Cps37LsVyPJL\nfjDlGIM5K3B/vtZfn2f8buQ9QyKiN0bc67GdCjih9dSrkQNkxJiEOwqiSjYtkdFO\ndYpXF8d1rQKV7a6z2vJloDwilfXLLlUX7rA3qVu7r4EUfIsZgH7hgB4bbst7tx+7\nPgUEq2334kKPVFpsxgsj5++k4lh7tNlakXiBUtw=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIEsTCCA5mgAwIBAgIQBOHnpNxc8vNtwCtCuF0VnzANBgkqhkiG9w0BAQsFADBs\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3\nd3cuZGlnaWNlcnQuY29tMSswKQYDVQQDEyJEaWdpQ2VydCBIaWdoIEFzc3VyYW5j\nZSBFViBSb290IENBMB4XDTEzMTAyMjEyMDAwMFoXDTI4MTAyMjEyMDAwMFowcDEL\nMAkGA1UEBhMCVVMxFTATBgNVBAoTDERpZ2lDZXJ0IEluYzEZMBcGA1UECxMQd3d3\nLmRpZ2ljZXJ0LmNvbTEvMC0GA1UEAxMmRGlnaUNlcnQgU0hBMiBIaWdoIEFzc3Vy\nYW5jZSBTZXJ2ZXIgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC2\n4C/CJAbIbQRf1+8KZAayfSImZRauQkCbztyfn3YHPsMwVYcZuU+UDlqUH1VWtMIC\nKq/QmO4LQNfE0DtyyBSe75CxEamu0si4QzrZCwvV1ZX1QK/IHe1NnF9Xt4ZQaJn1\nitrSxwUfqJfJ3KSxgoQtxq2lnMcZgqaFD15EWCo3j/018QsIJzJa9buLnqS9UdAn\n4t07QjOjBSjEuyjMmqwrIw14xnvmXnG3Sj4I+4G3FhahnSMSTeXXkgisdaScus0X\nsh5ENWV/UyU50RwKmmMbGZJ0aAo3wsJSSMs5WqK24V3B3aAguCGikyZvFEohQcft\nbZvySC/zA/WiaJJTL17jAgMBAAGjggFJMIIBRTASBgNVHRMBAf8ECDAGAQH/AgEA\nMA4GA1UdDwEB/wQEAwIBhjAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIw\nNAYIKwYBBQUHAQEEKDAmMCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2Vy\ndC5jb20wSwYDVR0fBEQwQjBAoD6gPIY6aHR0cDovL2NybDQuZGlnaWNlcnQuY29t\nL0RpZ2lDZXJ0SGlnaEFzc3VyYW5jZUVWUm9vdENBLmNybDA9BgNVHSAENjA0MDIG\nBFUdIAAwKjAoBggrBgEFBQcCARYcaHR0cHM6Ly93d3cuZGlnaWNlcnQuY29tL0NQ\nUzAdBgNVHQ4EFgQUUWj/kK8CB3U8zNllZGKiErhZcjswHwYDVR0jBBgwFoAUsT7D\naQP4v0cB1JgmGggC72NkK8MwDQYJKoZIhvcNAQELBQADggEBABiKlYkD5m3fXPwd\naOpKj4PWUS+Na0QWnqxj9dJubISZi6qBcYRb7TROsLd5kinMLYBq8I4g4Xmk/gNH\nE+r1hspZcX30BJZr01lYPf7TMSVcGDiEo+afgv2MW5gxTs14nhr9hctJqvIni5ly\n/D6q1UEL2tU2ob8cbkdJf17ZSHwD2f2LSaCYJkJA69aSEaRkCldUxPUd1gJea6zu\nxICaEnL6VpPX/78whQYwvwt/Tv9XBZ0k7YXDK/umdaisLRbvfXknsuvCnQsH6qqF\n0wGjIChBWUMo0oHjqvbsezt3tkBigAVBRQHvFwY+3sAzm2fTYS5yh+Rp/BIAV0Ae\ncPUeybQ=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIFWTCCBEGgAwIBAgIRAI2kTzBRQqhJ3nOm/ZZpYacwDQYJKoZIhvcNAQELBQAw\ngZAxCzAJBgNVBAYTAkdCMRswGQYDVQQIExJHcmVhdGVyIE1hbmNoZXN0ZXIxEDAO\nBgNVBAcTB1NhbGZvcmQxGjAYBgNVBAoTEUNPTU9ETyBDQSBMaW1pdGVkMTYwNAYD\nVQQDEy1DT01PRE8gUlNBIERvbWFpbiBWYWxpZGF0aW9uIFNlY3VyZSBTZXJ2ZXIg\nQ0EwHhcNMTYwNjI3MDAwMDAwWhcNMTcwNzI3MjM1OTU5WjBeMSEwHwYDVQQLExhE\nb21haW4gQ29udHJvbCBWYWxpZGF0ZWQxHjAcBgNVBAsTFUVzc2VudGlhbFNTTCBX\naWxkY2FyZDEZMBcGA1UEAwwQKi5jb250ZW50ZnVsLmNvbTCCASIwDQYJKoZIhvcN\nAQEBBQADggEPADCCAQoCggEBALCfMS7doJgi6LkkMuNxGyurtC8Vcm0GtOcWZuf3\nCwauhbwQSHIVxJ8ggcnoNmVXXJN1hqctFUpapt2JLAuwUQUc/k6QJY8M06nWytJI\np3Lf6o3bkWMBxbbIGV6L1ybmtBnh2lRCIw1MSnD620tEAH1om2UIgIPPI/6fH4ZC\n8P7S4/2ImJ9EsbGUuYoBPIP2pIcNMP+lRaIGpPqyffyP46Tr0gAhPC8SOctfRCRe\n5DjPkWTFCIK/X7wux5VWEKhk+ZmpN/E/930ixwZynNqGr/7GVWh4Vvqc7GgNb1yO\nl3co4xwSbdseCYL2eWWDrisP+h7KygIGKpZ116wjUjjClk0CAwEAAaOCAd0wggHZ\nMB8GA1UdIwQYMBaAFJCvajqUWgvYkOoSVnPfQ7Q6KNrnMB0GA1UdDgQWBBRSLKYh\nJNKm4ApiHS0i4VLS8TvSDzAOBgNVHQ8BAf8EBAMCBaAwDAYDVR0TAQH/BAIwADAd\nBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwTwYDVR0gBEgwRjA6BgsrBgEE\nAbIxAQICBzArMCkGCCsGAQUFBwIBFh1odHRwczovL3NlY3VyZS5jb21vZG8uY29t\nL0NQUzAIBgZngQwBAgEwVAYDVR0fBE0wSzBJoEegRYZDaHR0cDovL2NybC5jb21v\nZG9jYS5jb20vQ09NT0RPUlNBRG9tYWluVmFsaWRhdGlvblNlY3VyZVNlcnZlckNB\nLmNybDCBhQYIKwYBBQUHAQEEeTB3ME8GCCsGAQUFBzAChkNodHRwOi8vY3J0LmNv\nbW9kb2NhLmNvbS9DT01PRE9SU0FEb21haW5WYWxpZGF0aW9uU2VjdXJlU2VydmVy\nQ0EuY3J0MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5jb21vZG9jYS5jb20wKwYD\nVR0RBCQwIoIQKi5jb250ZW50ZnVsLmNvbYIOY29udGVudGZ1bC5jb20wDQYJKoZI\nhvcNAQELBQADggEBAGjTyCxabJc8vs4P2ayuF2/k8pr3oISpo8+bF4QFtXCQhr6I\n2G6OYvzZLWXCVFJ53FdT7PDIchP4tlYafySgXKo8POxfS20jBKk6+ZYEzwVlgRd2\njyhojQTNlilj9hPq3CJd4WK3KmA9Hnd9cRkdsduDcFeENviUWw/hgq3PvoYgGshh\nz9DzW878tMtAZk5DfiTkOvgphgvbaCod9W5MsDJ3NyQ6P88//28seyxs6yVTMKvM\nfsOz/kf3AgM+JbmAgpHZk8LkXI9qCIpjS9zcijRWz/QD8M/QedX5oKLB/PFBsP7k\n03x4tMWKes9Y3t+9Rdnx0kanOAkIzWLaZIh7igg=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIGCDCCA/CgAwIBAgIQKy5u6tl1NmwUim7bo3yMBzANBgkqhkiG9w0BAQwFADCB\nhTELMAkGA1UEBhMCR0IxGzAZBgNVBAgTEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4G\nA1UEBxMHU2FsZm9yZDEaMBgGA1UEChMRQ09NT0RPIENBIExpbWl0ZWQxKzApBgNV\nBAMTIkNPTU9ETyBSU0EgQ2VydGlmaWNhdGlvbiBBdXRob3JpdHkwHhcNMTQwMjEy\nMDAwMDAwWhcNMjkwMjExMjM1OTU5WjCBkDELMAkGA1UEBhMCR0IxGzAZBgNVBAgT\nEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4GA1UEBxMHU2FsZm9yZDEaMBgGA1UEChMR\nQ09NT0RPIENBIExpbWl0ZWQxNjA0BgNVBAMTLUNPTU9ETyBSU0EgRG9tYWluIFZh\nbGlkYXRpb24gU2VjdXJlIFNlcnZlciBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEP\nADCCAQoCggEBAI7CAhnhoFmk6zg1jSz9AdDTScBkxwtiBUUWOqigwAwCfx3M28Sh\nbXcDow+G+eMGnD4LgYqbSRutA776S9uMIO3Vzl5ljj4Nr0zCsLdFXlIvNN5IJGS0\nQa4Al/e+Z96e0HqnU4A7fK31llVvl0cKfIWLIpeNs4TgllfQcBhglo/uLQeTnaG6\nytHNe+nEKpooIZFNb5JPJaXyejXdJtxGpdCsWTWM/06RQ1A/WZMebFEh7lgUq/51\nUHg+TLAchhP6a5i84DuUHoVS3AOTJBhuyydRReZw3iVDpA3hSqXttn7IzW3uLh0n\nc13cRTCAquOyQQuvvUSH2rnlG51/ruWFgqUCAwEAAaOCAWUwggFhMB8GA1UdIwQY\nMBaAFLuvfgI9+qbxPISOre44mOzZMjLUMB0GA1UdDgQWBBSQr2o6lFoL2JDqElZz\n30O0Oija5zAOBgNVHQ8BAf8EBAMCAYYwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNV\nHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwGwYDVR0gBBQwEjAGBgRVHSAAMAgG\nBmeBDAECATBMBgNVHR8ERTBDMEGgP6A9hjtodHRwOi8vY3JsLmNvbW9kb2NhLmNv\nbS9DT01PRE9SU0FDZXJ0aWZpY2F0aW9uQXV0aG9yaXR5LmNybDBxBggrBgEFBQcB\nAQRlMGMwOwYIKwYBBQUHMAKGL2h0dHA6Ly9jcnQuY29tb2RvY2EuY29tL0NPTU9E\nT1JTQUFkZFRydXN0Q0EuY3J0MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5jb21v\nZG9jYS5jb20wDQYJKoZIhvcNAQEMBQADggIBAE4rdk+SHGI2ibp3wScF9BzWRJ2p\nmj6q1WZmAT7qSeaiNbz69t2Vjpk1mA42GHWx3d1Qcnyu3HeIzg/3kCDKo2cuH1Z/\ne+FE6kKVxF0NAVBGFfKBiVlsit2M8RKhjTpCipj4SzR7JzsItG8kO3KdY3RYPBps\nP0/HEZrIqPW1N+8QRcZs2eBelSaz662jue5/DJpmNXMyYE7l3YphLG5SEXdoltMY\ndVEVABt0iN3hxzgEQyjpFv3ZBdRdRydg1vs4O2xyopT4Qhrf7W8GjEXCBgCq5Ojc\n2bXhc3js9iPc0d1sjhqPpepUfJa3w/5Vjo1JXvxku88+vZbrac2/4EjxYoIQ5QxG\nV/Iz2tDIY+3GH5QFlkoakdH368+PUq4NCNk+qKBR6cGHdNXJ93SrLlP7u3r7l+L4\nHyaPs9Kg4DdbKDsx5Q5XLVq4rXmsXiBmGqW5prU5wfWYQ//u+aen/e7KJD2AFsQX\nj4rBYKEMrltDR5FL1ZoXX/nUh8HCjLfn4g8wGTeGrODcQgPmlKidrv0PJFGUzpII\n0fxQ8ANAe4hZ7Q7drNJ3gjTcBpUC2JD5Leo31Rpg0Gcg19hCC0Wvgmje3WYkN5Ap\nlBlGGSW4gNfL1IYoakRwJiNiqZ+Gb7+6kHDSVneFeO/qJakXzlByjAA6quPbYzSf\n+AZxAeKCINT+b72x\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIFdDCCBFygAwIBAgIQJ2buVutJ846r13Ci/ITeIjANBgkqhkiG9w0BAQwFADBv\nMQswCQYDVQQGEwJTRTEUMBIGA1UEChMLQWRkVHJ1c3QgQUIxJjAkBgNVBAsTHUFk\nZFRydXN0IEV4dGVybmFsIFRUUCBOZXR3b3JrMSIwIAYDVQQDExlBZGRUcnVzdCBF\neHRlcm5hbCBDQSBSb290MB4XDTAwMDUzMDEwNDgzOFoXDTIwMDUzMDEwNDgzOFow\ngYUxCzAJBgNVBAYTAkdCMRswGQYDVQQIExJHcmVhdGVyIE1hbmNoZXN0ZXIxEDAO\nBgNVBAcTB1NhbGZvcmQxGjAYBgNVBAoTEUNPTU9ETyBDQSBMaW1pdGVkMSswKQYD\nVQQDEyJDT01PRE8gUlNBIENlcnRpZmljYXRpb24gQXV0aG9yaXR5MIICIjANBgkq\nhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAkehUktIKVrGsDSTdxc9EZ3SZKzejfSNw\nAHG8U9/E+ioSj0t/EFa9n3Byt2F/yUsPF6c947AEYe7/EZfH9IY+Cvo+XPmT5jR6\n2RRr55yzhaCCenavcZDX7P0N+pxs+t+wgvQUfvm+xKYvT3+Zf7X8Z0NyvQwA1onr\nayzT7Y+YHBSrfuXjbvzYqOSSJNpDa2K4Vf3qwbxstovzDo2a5JtsaZn4eEgwRdWt\n4Q08RWD8MpZRJ7xnw8outmvqRsfHIKCxH2XeSAi6pE6p8oNGN4Tr6MyBSENnTnIq\nm1y9TBsoilwie7SrmNnu4FGDwwlGTm0+mfqVF9p8M1dBPI1R7Qu2XK8sYxrfV8g/\nvOldxJuvRZnio1oktLqpVj3Pb6r/SVi+8Kj/9Lit6Tf7urj0Czr56ENCHonYhMsT\n8dm74YlguIwoVqwUHZwK53Hrzw7dPamWoUi9PPevtQ0iTMARgexWO/bTouJbt7IE\nIlKVgJNp6I5MZfGRAy1wdALqi2cVKWlSArvX31BqVUa/oKMoYX9w0MOiqiwhqkfO\nKJwGRXa/ghgntNWutMtQ5mv0TIZxMOmm3xaG4Nj/QN370EKIf6MzOi5cHkERgWPO\nGHFrK+ymircxXDpqR+DDeVnWIBqv8mqYqnK8V0rSS527EPywTEHl7R09XiidnMy/\ns1Hap0flhFMCAwEAAaOB9DCB8TAfBgNVHSMEGDAWgBStvZh6NLQm9/rEJlTvA73g\nJMtUGjAdBgNVHQ4EFgQUu69+Aj36pvE8hI6t7jiY7NkyMtQwDgYDVR0PAQH/BAQD\nAgGGMA8GA1UdEwEB/wQFMAMBAf8wEQYDVR0gBAowCDAGBgRVHSAAMEQGA1UdHwQ9\nMDswOaA3oDWGM2h0dHA6Ly9jcmwudXNlcnRydXN0LmNvbS9BZGRUcnVzdEV4dGVy\nbmFsQ0FSb290LmNybDA1BggrBgEFBQcBAQQpMCcwJQYIKwYBBQUHMAGGGWh0dHA6\nLy9vY3NwLnVzZXJ0cnVzdC5jb20wDQYJKoZIhvcNAQEMBQADggEBAGS/g/FfmoXQ\nzbihKVcN6Fr30ek+8nYEbvFScLsePP9NDXRqzIGCJdPDoCpdTPW6i6FtxFQJdcfj\nJw5dhHk3QBN39bSsHNA7qxcS1u80GH4r6XnTq1dFDK8o+tDb5VCViLvfhVdpfZLY\nUspzgb8c8+a4bmYRBbMelC1/kZWSWfFMzqORcUx8Rww7Cxn2obFshj5cqsQugsv5\nB5a6SE2Q8pTIqXOi6wZ7I53eovNNVZ96YUWYGGjHXkBrI/V5eu+MtWuLt29G9Hvx\nPUsE2JOAWVrgQSQdso8VYFhH2+9uRv0V9dlfmrPb2LjkQLPNlzmuhbsdjrzch5vR\npu/xO28QOG8=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIENjCCAx6gAwIBAgIBATANBgkqhkiG9w0BAQUFADBvMQswCQYDVQQGEwJTRTEU\nMBIGA1UEChMLQWRkVHJ1c3QgQUIxJjAkBgNVBAsTHUFkZFRydXN0IEV4dGVybmFs\nIFRUUCBOZXR3b3JrMSIwIAYDVQQDExlBZGRUcnVzdCBFeHRlcm5hbCBDQSBSb290\nMB4XDTAwMDUzMDEwNDgzOFoXDTIwMDUzMDEwNDgzOFowbzELMAkGA1UEBhMCU0Ux\nFDASBgNVBAoTC0FkZFRydXN0IEFCMSYwJAYDVQQLEx1BZGRUcnVzdCBFeHRlcm5h\nbCBUVFAgTmV0d29yazEiMCAGA1UEAxMZQWRkVHJ1c3QgRXh0ZXJuYWwgQ0EgUm9v\ndDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALf3GjPm8gAELTngTlvt\nH7xsD821+iO2zt6bETOXpClMfZOfvUq8k+0DGuOPz+VtUFrWlymUWoCwSXrbLpX9\nuMq/NzgtHj6RQa1wVsfwTz/oMp50ysiQVOnGXw94nZpAPA6sYapeFI+eh6FqUNzX\nmk6vBbOmcZSccbNQYArHE504B4YCqOmoaSYYkKtMsE8jqzpPhNjfzp/haW+710LX\na0Tkx63ubUFfclpxCDezeWWkWaCUN/cALw3CknLa0Dhy2xSoRcRdKn23tNbE7qzN\nE0S3ySvdQwAl+mG5aWpYIxG3pzOPVnVZ9c0p10a3CitlttNCbxWyuHv77+ldU9U0\nWicCAwEAAaOB3DCB2TAdBgNVHQ4EFgQUrb2YejS0Jvf6xCZU7wO94CTLVBowCwYD\nVR0PBAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wgZkGA1UdIwSBkTCBjoAUrb2YejS0\nJvf6xCZU7wO94CTLVBqhc6RxMG8xCzAJBgNVBAYTAlNFMRQwEgYDVQQKEwtBZGRU\ncnVzdCBBQjEmMCQGA1UECxMdQWRkVHJ1c3QgRXh0ZXJuYWwgVFRQIE5ldHdvcmsx\nIjAgBgNVBAMTGUFkZFRydXN0IEV4dGVybmFsIENBIFJvb3SCAQEwDQYJKoZIhvcN\nAQEFBQADggEBALCb4IUlwtYj4g+WBpKdQZic2YR5gdkeWxQHIzZlj7DYd7usQWxH\nYINRsPkyPef89iYTx4AWpb9a/IfPeHmJIZriTAcKhjW88t5RxNKWt9x+Tu5w/Rw5\n6wwCURQtjr0W4MHfRnXnJK3s9EK0hZNwEGe6nQY1ShjTK3rMUUKhemPR5ruhxSvC\nNr4TDea9Y355e6cJDUCrat2PisP29owaQgVR1EX1n6diIWgVIEM8med8vSTYqZEX\nc4g/VhsxOBi0cQ+azcgOno4uG+GMmIPLHzHxREzGBHNJdmAPx/i9F4BrLunMTA5a\nmnkPIAou1Z5jJh5VkpTYghdae9C8x49OhgQ=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIKuzCCCaOgAwIBAgIMNjYXNLLXqLY/TOgcMA0GCSqGSIb3DQEBCwUAMFcxCzAJ\nBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9iYWxTaWduIG52LXNhMS0wKwYDVQQDEyRH\nbG9iYWxTaWduIENsb3VkU1NMIENBIC0gU0hBMjU2IC0gRzMwHhcNMTcwMzIzMTEy\nMzE5WhcNMTcxMTAzMTEyMjIzWjBgMQswCQYDVQQGEwJVUzERMA8GA1UECBMIRGVs\nYXdhcmUxDjAMBgNVBAcTBURvdmVyMRYwFAYDVQQKEw1JbmNhcHN1bGEgSW5jMRYw\nFAYDVQQDEw1pbmNhcHN1bGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB\nCgKCAQEA3sgLKGR+l4KI2c5rjhwTQcuXu32CXy2BuUqZkco4pDsHzvBWKNAzxqSe\nUOt4LmhBzLETdHqvsXVPnNW4mbruVbreTZoJ6//Jy4WfBAtvVq/uL9krmv19opt1\n0Ll4+LLpeI+VBnkCNBPrALk/7aoVHeZcTmRmq2QPbRIBSaa+ZQhL+swQc+ATmcxd\nz9ew1gZPo9kWC4lZ63FIRu97wa3yKWi1qD8alv1EKFgInZOLtp359BOJlLvTt4zC\nnAzvMZog8YXJ9Mit5mSWTBGfKp0Z4QF5GIQNcM2rPFZC3nWjwjp7DwgsCvpJDzHR\nkpULclhrBX7LvbvMdMzT2KgvvrwZtwIDAQABo4IHfDCCB3gwDgYDVR0PAQH/BAQD\nAgWgMIGKBggrBgEFBQcBAQR+MHwwQgYIKwYBBQUHMAKGNmh0dHA6Ly9zZWN1cmUu\nZ2xvYmFsc2lnbi5jb20vY2FjZXJ0L2Nsb3Vkc3Nsc2hhMmczLmNydDA2BggrBgEF\nBQcwAYYqaHR0cDovL29jc3AyLmdsb2JhbHNpZ24uY29tL2Nsb3Vkc3Nsc2hhMmcz\nMFYGA1UdIARPME0wQQYJKwYBBAGgMgEUMDQwMgYIKwYBBQUHAgEWJmh0dHBzOi8v\nd3d3Lmdsb2JhbHNpZ24uY29tL3JlcG9zaXRvcnkvMAgGBmeBDAECAjAJBgNVHRME\nAjAAMIIGFQYDVR0RBIIGDDCCBgiCDWluY2Fwc3VsYS5jb22CHSouYWNjZXB0YXRp\nZS1lbmdpZS1lbmVyZ2llLm5sgg0qLmFtd2FsYWsuY29tghsqLmFwcGx5LmdvbWFz\ndGVyY2FyZC5jb20uYXWCCyouYXZpdmEuY29tghEqLmF2aXZhY2FuYWRhLmNvbYIQ\nKi5iaW5nb21hbmlhLmNvbYIRKi5icmFuY2hldHZvdXMuZnKCEyouY2xlYXJza3kt\nZGF0YS5uZXSCDiouY29uZWN0eXMuY29tghAqLmNvbnRlbnRmdWwuY29tgg4qLmNv\ncmVmb3VyLmNvbYIMKi5jb3JzYWlyLmNpggwqLmNvcnNhaXIuZ3CCDCouY29yc2Fp\nci5tcYIMKi5jb3JzYWlyLnNughQqLmNyZWRpdG95Y2F1Y2lvbi5lc4IWKi5kYzQu\ncGFnZXVwcGVvcGxlLmNvbYIWKi5kZXZpY2Vwcm90ZWN0aW9uLmNvbYISKi5kaWFq\ndWdvc28xMjMuY29tghUqLmRpcmVjdG1vYmlsZXMuY28udWuCEyouZWRkaWVhbmRj\nby5jb20uYXWCFCouZWtlZXBlcmdyb3VwLmNvLnVrggwqLmVwaWR1by5jb22CESou\nZXBpZHVvZm9ydGUuY29tghUqLmV2b3F1YWFkdmFudGFnZS5jb22CFSouaGVkZ2Vz\ndG9uZWdyb3VwLmNvbYIUKi5pbmNhcHN1bGEtZGVtby5iaXqCDCouaXZyYXBwLm5l\ndIINKi5rcC1teXBnLmNvbYIYKi5sYi5uZXN0bGUtd2F0ZXJzbmEuY29tgg0qLmx1\nbHVjcm0uY29tghMqLm1hZGV3aXRobmVzdGxlLmNhgg4qLm1hcmljb3BhLmVkdYIU\nKi5tYXR0ZWxwYXJ0bmVycy5jb22CGioubXhwLnphbXNoLmluY2Fwc3VsYS5tb2Jp\ngg0qLm15ZXIuY29tLmF1gg0qLm15bnZhcHAuY29tgg8qLm5ldDJwaG9uZS5jb22C\nDSoucGluMTExMS5jb22CECoucHJvZC5pbG9hbi5jb22CGCouc2Nhbm5lci5zcG90\nb3B0aW9uLmNvbYIZKi5zZWFyY2hmbG93c3RhZ2luZy5jby51a4IXKi5zaW1wbHli\nZXR0ZXJ0aW4uY28udWuCCiouc29mbi5jb22CDiouc3RyYXR0b24uY29tghcqLnRl\nc3QtZW5naWUtZW5lcmdpZS5ubIIQKi50cmF2ZWxwb3J0LmNvbYIOKi50cmVtYmxh\nbnQuY2GCCyoudml0dGVsLmpwgg0qLnZ0ZWNoLmNvLnVrgg0qLndlcmFsbHkuY29t\nghcqLndoaXRlaG91c2VoaXN0b3J5Lm9yZ4IMKi53cnBzLm9uLmNhggkqLnd0ZS5u\nZXSCCyoueW91ZmkuY29tghthY2NlcHRhdGllLWVuZ2llLWVuZXJnaWUubmyCC2Ft\nd2FsYWsuY29tgglhdml2YS5jb22CDmJpbmdvbWFuaWEuY29tgg9icmFuY2hldHZv\ndXMuZnKCDGNvbmVjdHlzLmNvbYIKY29yc2Fpci5jaYIKY29yc2Fpci5ncIIKY29y\nc2Fpci5tcYIKY29yc2Fpci5zboIUZGV2aWNlcHJvdGVjdGlvbi5jb22CEGRpYWp1\nZ29zbzEyMy5jb22CE2RpcmVjdG1vYmlsZXMuY28udWuCEWVkZGllYW5kY28uY29t\nLmF1ggplcGlkdW8uY29tgg9lcGlkdW9mb3J0ZS5jb22CE2V2b3F1YWFkdmFudGFn\nZS5jb22CE2hlZGdlc3RvbmVncm91cC5jb22CC2twLW15cGcuY29tghFtYWRld2l0\naG5lc3RsZS5jYYINbmV0MnBob25lLmNvbYILcGluMTExMS5jb22CF3NlYXJjaGZs\nb3dzdGFnaW5nLmNvLnVrghVzaW1wbHliZXR0ZXJ0aW4uY28udWuCFXRlc3QtZW5n\naWUtZW5lcmdpZS5ubIIOdHJhdmVscG9ydC5jb22CC3Z0ZWNoLmNvLnVrggt3ZXJh\nbGx5LmNvbYIKd3Jwcy5vbi5jYYIHd3RlLm5ldDAdBgNVHSUEFjAUBggrBgEFBQcD\nAQYIKwYBBQUHAwIwHQYDVR0OBBYEFFAxG/f60PL3CFmmnW/Uk9gBtDhCMB8GA1Ud\nIwQYMBaAFKkrh+HOJEc7G7/PhTcCVZ0NlFjmMA0GCSqGSIb3DQEBCwUAA4IBAQAn\n/+V5a6hfMgXZJa5H0c0Gu5E3bSl8gvQsS4VsT1tnI0OnjwqQtd4fRC2TQFogSYbk\nDfmtFxiiQymF9CtlRbTQX41gZ6RMLRIyeA96k7WC3PJiIlqiFDp0172INTU0NsiX\nfJ/u2plLINtye67yUt38TGOYZa1aF/mjAtN+tubumY1Va7k/ec4b+qhZQUOzgGZv\nTrx5wDYy8UBeUkqn7/ZVH7FDAN6wcc97e49/02okiH7pu9bSlP9Izl6YSaLBRcAy\nTAixepqGxVjhK1OmKMIGrIiI2H9oEelpNImMvgQo8XK+2Bcvnw/H5qQg8VmFwPd7\ndfeszhlA0vV4mgGQMm5T\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIEizCCA3OgAwIBAgIORvCM288sVGbvMwHdXzQwDQYJKoZIhvcNAQELBQAwVzEL\nMAkGA1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsT\nB1Jvb3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw0xNTA4MTkw\nMDAwMDBaFw0yNTA4MTkwMDAwMDBaMFcxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBH\nbG9iYWxTaWduIG52LXNhMS0wKwYDVQQDEyRHbG9iYWxTaWduIENsb3VkU1NMIENB\nIC0gU0hBMjU2IC0gRzMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCj\nwHXhMpjl2a6EfI3oI19GlVtMoiVw15AEhYDJtfSKZU2Sy6XEQqC2eSUx7fGFIM0T\nUT1nrJdNaJszhlyzey2q33egYdH1PPua/NPVlMrJHoAbkJDIrI32YBecMbjFYaLi\nblclCG8kmZnPlL/Hi2uwH8oU+hibbBB8mSvaSmPlsk7C/T4QC0j0dwsv8JZLOu69\nNd6FjdoTDs4BxHHT03fFCKZgOSWnJ2lcg9FvdnjuxURbRb0pO+LGCQ+ivivc41za\nWm+O58kHa36hwFOVgongeFxyqGy+Z2ur5zPZh/L4XCf09io7h+/awkfav6zrJ2R7\nTFPrNOEvmyBNVBJrfSi9AgMBAAGjggFTMIIBTzAOBgNVHQ8BAf8EBAMCAQYwHQYD\nVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMBIGA1UdEwEB/wQIMAYBAf8CAQAw\nHQYDVR0OBBYEFKkrh+HOJEc7G7/PhTcCVZ0NlFjmMB8GA1UdIwQYMBaAFGB7ZhpF\nDZfKiVAvfQTNNKj//P1LMD0GCCsGAQUFBwEBBDEwLzAtBggrBgEFBQcwAYYhaHR0\ncDovL29jc3AuZ2xvYmFsc2lnbi5jb20vcm9vdHIxMDMGA1UdHwQsMCowKKAmoCSG\nImh0dHA6Ly9jcmwuZ2xvYmFsc2lnbi5jb20vcm9vdC5jcmwwVgYDVR0gBE8wTTAL\nBgkrBgEEAaAyARQwPgYGZ4EMAQICMDQwMgYIKwYBBQUHAgEWJmh0dHBzOi8vd3d3\nLmdsb2JhbHNpZ24uY29tL3JlcG9zaXRvcnkvMA0GCSqGSIb3DQEBCwUAA4IBAQCi\nHWmKCo7EFIMqKhJNOSeQTvCNrNKWYkc2XpLR+sWTtTcHZSnS9FNQa8n0/jT13bgd\n+vzcFKxWlCecQqoETbftWNmZ0knmIC/Tp3e4Koka76fPhi3WU+kLk5xOq9lF7qSE\nhf805A7Au6XOX5WJhXCqwV3szyvT2YPfA8qBpwIyt3dhECVO2XTz2XmCtSZwtFK8\njzPXiq4Z0PySrS+6PKBIWEde/SBWlSDBch2rZpmk1Xg3SBufskw3Z3r9QtLTVp7T\nHY7EDGiWtkdREPd76xUJZPX58GMWLT3fI0I6k2PMq69PVwbH/hRVYs4nERnh9ELt\nIjBrNRpKBYCkZd/My2/Q\n-----END CERTIFICATE-----\n"))
	return &ContentClient{
		Locale:    locale,
		authToken: authToken,
		client:    &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}},
		host:      fmt.Sprintf("https://%s", contentfulCDAURL),
		pool:      pool,
		spaceID:   "ygx37epqlss8",
	}
}

// NewCPA returns a contentful client interfacing with the content preview api
func NewCPA(authToken string, locale string) *ContentClient {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte("-----BEGIN CERTIFICATE-----\nMIIL6TCCCtGgAwIBAgIQBigdNnW0H8yz/xj67Pj93zANBgkqhkiG9w0BAQsFADBw\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3\nd3cuZGlnaWNlcnQuY29tMS8wLQYDVQQDEyZEaWdpQ2VydCBTSEEyIEhpZ2ggQXNz\ndXJhbmNlIFNlcnZlciBDQTAeFw0xNDEyMDgwMDAwMDBaFw0xODAyMDYxMjAwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRUwEwYDVQQKEwxGYXN0bHksIEluYy4xGTAXBgNVBAMTEGEu\nc3NsLmZhc3RseS5uZXQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDU\nJUiQsaVP/vC4Mb3aJUmA9KnMQa7EJfjYLsE4F0VehrOp8jlSSXmQLELlUAwPp2F2\nPNyB32DDOFBHZIYwFrApFEzsJdTKQUYk6xHPZOdYoIijpmfb5xRMdTjqxThGkk+k\nhU0+ipPWiErJNRkapLgPwPD4ctd5X8rnKF8lMHIxx5Xhdg6PqZC3F7y45Nym2a3M\n8xIKIkB77o1bkuDpGnV9ZESC/Yf9Mc4NmWrQjqQc+8yIabir+n7/YcM5UdUjZPNS\nhgL4jLYVJ+KDRZcjIT/dXRZoPpJgRFL9NIep/eSAzQa3g659uW7tjN6tg5iQm4hw\nksaWp+zfTAJc4IXNtlndAgMBAAGjggiBMIIIfTAfBgNVHSMEGDAWgBRRaP+QrwIH\ndTzM2WVkYqISuFlyOzAdBgNVHQ4EFgQUwIj0Y03ka1Q28RLCtKWy4nN7FIgwggax\nBgNVHREEggaoMIIGpIIQYS5zc2wuZmFzdGx5Lm5ldIISKi5hLnNzbC5mYXN0bHku\nbmV0gg9mYXN0Lndpc3RpYS5jb22CEHB1cmdlLmZhc3RseS5uZXSCEm1pcnJvcnMu\nZmFzdGx5Lm5ldIIOKi5wYXJzZWNkbi5jb22CDSouZmFzdHNzbC5uZXSCCXZveGVy\nLmNvbYINd3d3LnZveGVyLmNvbYIOKi5maXJlYmFzZS5jb22CEHNpdGVzLnlhbW1l\nci5jb22CGHNpdGVzLnN0YWdpbmcueWFtbWVyLmNvbYIPKi5za2ltbGlua3MuY29t\nghMqLnNraW1yZXNvdXJjZXMuY29tghBjZG4udGhpbmdsaW5rLm1lggwqLmZpdGJp\ndC5jb22CEiouaG9zdHMuZmFzdGx5Lm5ldIISY29udHJvbC5mYXN0bHkubmV0gg8q\nLndpa2lhLWluYy5jb22CFSoucGVyZmVjdGF1ZGllbmNlLmNvbYILKi53aWtpYS5j\nb22CEmYuY2xvdWQuZ2l0aHViLmNvbYIVKi5kaWdpdGFsc2Npcm9jY28ubmV0ggoq\nLmV0c3kuY29tghAqLmV0c3lzdGF0aWMuY29tgg0qLmFkZHRoaXMuY29tghAqLmFk\nZHRoaXNjZG4uY29tgg9mYXN0Lndpc3RpYS5uZXSCDnJhdy5naXRodWIuY29tgg93\nd3cudXNlcmZveC5jb22CEyouYXNzZXRzLXlhbW1lci5jb22CGyouc3RhZ2luZy5h\nc3NldHMteWFtbWVyLmNvbYIWYXNzZXRzLmh1Z2dpZXMtY2RuLm5ldIISb3JiaXQu\nc2hhemFtaWQuY29tgg9hYm91dC5qc3Rvci5vcmeCFyouZ2xvYmFsLnNzbC5mYXN0\nbHkubmV0gg13ZWIudm94ZXIuY29tgg9weXBpLnB5dGhvbi5vcmeCCyouMTJ3YnQu\nY29tghJ3d3cuaG9sZGVyZGVvcmQubm+CGnNlY3VyZWQuaW5kbi5pbmZvbGlua3Mu\nY29tghBwbGF5LnZpZHlhcmQuY29tghhwbGF5LXN0YWdpbmcudmlkeWFyZC5jb22C\nFXNlY3VyZS5pbWcud2ZyY2RuLmNvbYIWc2VjdXJlLmltZy5qb3NzY2RuLmNvbYIQ\nKi5nb2NhcmRsZXNzLmNvbYIVd2lkZ2V0cy5waW50ZXJlc3QuY29tgg4qLjdkaWdp\ndGFsLmNvbYINKi43c3RhdGljLmNvbYIPcC5kYXRhZG9naHEuY29tghBuZXcubXVs\nYmVycnkuY29tghJ3d3cuc2FmYXJpZmxvdy5jb22CEmNkbi5jb250ZW50ZnVsLmNv\nbYIQdG9vbHMuZmFzdGx5Lm5ldIISKi5odWV2b3NidWVub3MuY29tgg4qLmdvb2Rl\nZ2dzLmNvbYIWKi5mYXN0bHkucGljbW9ua2V5LmNvbYIVKi5jZG4ud2hpcHBsZWhp\nbGwubmV0ghEqLndoaXBwbGVoaWxsLm5ldIIbY2RuLm1lZGlhMzQud2hpcHBsZWhp\nbGwubmV0ghtjZG4ubWVkaWE1Ni53aGlwcGxlaGlsbC5uZXSCG2Nkbi5tZWRpYTc4\nLndoaXBwbGVoaWxsLm5ldIIcY2RuLm1lZGlhOTEwLndoaXBwbGVoaWxsLm5ldIIO\nKi5tb2RjbG90aC5jb22CDyouZGlzcXVzY2RuLmNvbYILKi5qc3Rvci5vcmeCDyou\nZHJlYW1ob3N0LmNvbYIOd3d3LmZsaW50by5jb22CDyouY2hhcnRiZWF0LmNvbYIN\nKi5oaXBtdW5rLmNvbYIaY29udGVudC5iZWF2ZXJicm9va3MuY28udWuCG3NlY3Vy\nZS5jb21tb24uY3Nuc3RvcmVzLmNvbYIOd3d3LmpvaW5vcy5jb22CJXN0YWdpbmct\nbW9iaWxlLWNvbGxlY3Rvci5uZXdyZWxpYy5jb22CDioubW9kY2xvdGgubmV0ghAq\nLmZvdXJzcXVhcmUuY29tggwqLnNoYXphbS5jb22CCiouNHNxaS5uZXSCDioubWV0\nYWNwYW4ub3JnggwqLmZhc3RseS5jb22CCXdpa2lhLmNvbYIKZmFzdGx5LmNvbYIR\nKi5nYWR2ZW50dXJlcy5jb22CFnd3dy5nYWR2ZW50dXJlcy5jb20uYXWCFXd3dy5n\nYWR2ZW50dXJlcy5jby51a4IJa3JlZG8uY29tghZjZG4tdGFncy5icmFpbmllbnQu\nY29tghRteS5iaWxsc3ByaW5nYXBwLmNvbYIGcnZtLmlvMA4GA1UdDwEB/wQEAwIF\noDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwdQYDVR0fBG4wbDA0oDKg\nMIYuaHR0cDovL2NybDMuZGlnaWNlcnQuY29tL3NoYTItaGEtc2VydmVyLWc1LmNy\nbDA0oDKgMIYuaHR0cDovL2NybDQuZGlnaWNlcnQuY29tL3NoYTItaGEtc2VydmVy\nLWc1LmNybDBMBgNVHSAERTBDMDcGCWCGSAGG/WwBATAqMCgGCCsGAQUFBwIBFhxo\ndHRwczovL3d3dy5kaWdpY2VydC5jb20vQ1BTMAgGBmeBDAECAjCBgwYIKwYBBQUH\nAQEEdzB1MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2VydC5jb20wTQYI\nKwYBBQUHMAKGQWh0dHA6Ly9jYWNlcnRzLmRpZ2ljZXJ0LmNvbS9EaWdpQ2VydFNI\nQTJIaWdoQXNzdXJhbmNlU2VydmVyQ0EuY3J0MAwGA1UdEwEB/wQCMAAwDQYJKoZI\nhvcNAQELBQADggEBAKLWzbX7wSyjzE7BVMjLrHAaiz+WGSwrAPrQBJ29sqouu9gv\nI7i2Ie6eiRb4YLMouy6D+ZNZ+RM+Hkjv+PZFxCcDRmaWi+74ha5d8O155gRJRPZ0\nSy5SfD/8kqrJRfC+/D/KdQzOroD4sx6Qprs9lZ0IEn4CTf0YPNV+Cps37LsVyPJL\nfjDlGIM5K3B/vtZfn2f8buQ9QyKiN0bc67GdCjih9dSrkQNkxJiEOwqiSjYtkdFO\ndYpXF8d1rQKV7a6z2vJloDwilfXLLlUX7rA3qVu7r4EUfIsZgH7hgB4bbst7tx+7\nPgUEq2334kKPVFpsxgsj5++k4lh7tNlakXiBUtw=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIEsTCCA5mgAwIBAgIQBOHnpNxc8vNtwCtCuF0VnzANBgkqhkiG9w0BAQsFADBs\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3\nd3cuZGlnaWNlcnQuY29tMSswKQYDVQQDEyJEaWdpQ2VydCBIaWdoIEFzc3VyYW5j\nZSBFViBSb290IENBMB4XDTEzMTAyMjEyMDAwMFoXDTI4MTAyMjEyMDAwMFowcDEL\nMAkGA1UEBhMCVVMxFTATBgNVBAoTDERpZ2lDZXJ0IEluYzEZMBcGA1UECxMQd3d3\nLmRpZ2ljZXJ0LmNvbTEvMC0GA1UEAxMmRGlnaUNlcnQgU0hBMiBIaWdoIEFzc3Vy\nYW5jZSBTZXJ2ZXIgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC2\n4C/CJAbIbQRf1+8KZAayfSImZRauQkCbztyfn3YHPsMwVYcZuU+UDlqUH1VWtMIC\nKq/QmO4LQNfE0DtyyBSe75CxEamu0si4QzrZCwvV1ZX1QK/IHe1NnF9Xt4ZQaJn1\nitrSxwUfqJfJ3KSxgoQtxq2lnMcZgqaFD15EWCo3j/018QsIJzJa9buLnqS9UdAn\n4t07QjOjBSjEuyjMmqwrIw14xnvmXnG3Sj4I+4G3FhahnSMSTeXXkgisdaScus0X\nsh5ENWV/UyU50RwKmmMbGZJ0aAo3wsJSSMs5WqK24V3B3aAguCGikyZvFEohQcft\nbZvySC/zA/WiaJJTL17jAgMBAAGjggFJMIIBRTASBgNVHRMBAf8ECDAGAQH/AgEA\nMA4GA1UdDwEB/wQEAwIBhjAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIw\nNAYIKwYBBQUHAQEEKDAmMCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2Vy\ndC5jb20wSwYDVR0fBEQwQjBAoD6gPIY6aHR0cDovL2NybDQuZGlnaWNlcnQuY29t\nL0RpZ2lDZXJ0SGlnaEFzc3VyYW5jZUVWUm9vdENBLmNybDA9BgNVHSAENjA0MDIG\nBFUdIAAwKjAoBggrBgEFBQcCARYcaHR0cHM6Ly93d3cuZGlnaWNlcnQuY29tL0NQ\nUzAdBgNVHQ4EFgQUUWj/kK8CB3U8zNllZGKiErhZcjswHwYDVR0jBBgwFoAUsT7D\naQP4v0cB1JgmGggC72NkK8MwDQYJKoZIhvcNAQELBQADggEBABiKlYkD5m3fXPwd\naOpKj4PWUS+Na0QWnqxj9dJubISZi6qBcYRb7TROsLd5kinMLYBq8I4g4Xmk/gNH\nE+r1hspZcX30BJZr01lYPf7TMSVcGDiEo+afgv2MW5gxTs14nhr9hctJqvIni5ly\n/D6q1UEL2tU2ob8cbkdJf17ZSHwD2f2LSaCYJkJA69aSEaRkCldUxPUd1gJea6zu\nxICaEnL6VpPX/78whQYwvwt/Tv9XBZ0k7YXDK/umdaisLRbvfXknsuvCnQsH6qqF\n0wGjIChBWUMo0oHjqvbsezt3tkBigAVBRQHvFwY+3sAzm2fTYS5yh+Rp/BIAV0Ae\ncPUeybQ=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIFWTCCBEGgAwIBAgIRAI2kTzBRQqhJ3nOm/ZZpYacwDQYJKoZIhvcNAQELBQAw\ngZAxCzAJBgNVBAYTAkdCMRswGQYDVQQIExJHcmVhdGVyIE1hbmNoZXN0ZXIxEDAO\nBgNVBAcTB1NhbGZvcmQxGjAYBgNVBAoTEUNPTU9ETyBDQSBMaW1pdGVkMTYwNAYD\nVQQDEy1DT01PRE8gUlNBIERvbWFpbiBWYWxpZGF0aW9uIFNlY3VyZSBTZXJ2ZXIg\nQ0EwHhcNMTYwNjI3MDAwMDAwWhcNMTcwNzI3MjM1OTU5WjBeMSEwHwYDVQQLExhE\nb21haW4gQ29udHJvbCBWYWxpZGF0ZWQxHjAcBgNVBAsTFUVzc2VudGlhbFNTTCBX\naWxkY2FyZDEZMBcGA1UEAwwQKi5jb250ZW50ZnVsLmNvbTCCASIwDQYJKoZIhvcN\nAQEBBQADggEPADCCAQoCggEBALCfMS7doJgi6LkkMuNxGyurtC8Vcm0GtOcWZuf3\nCwauhbwQSHIVxJ8ggcnoNmVXXJN1hqctFUpapt2JLAuwUQUc/k6QJY8M06nWytJI\np3Lf6o3bkWMBxbbIGV6L1ybmtBnh2lRCIw1MSnD620tEAH1om2UIgIPPI/6fH4ZC\n8P7S4/2ImJ9EsbGUuYoBPIP2pIcNMP+lRaIGpPqyffyP46Tr0gAhPC8SOctfRCRe\n5DjPkWTFCIK/X7wux5VWEKhk+ZmpN/E/930ixwZynNqGr/7GVWh4Vvqc7GgNb1yO\nl3co4xwSbdseCYL2eWWDrisP+h7KygIGKpZ116wjUjjClk0CAwEAAaOCAd0wggHZ\nMB8GA1UdIwQYMBaAFJCvajqUWgvYkOoSVnPfQ7Q6KNrnMB0GA1UdDgQWBBRSLKYh\nJNKm4ApiHS0i4VLS8TvSDzAOBgNVHQ8BAf8EBAMCBaAwDAYDVR0TAQH/BAIwADAd\nBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwTwYDVR0gBEgwRjA6BgsrBgEE\nAbIxAQICBzArMCkGCCsGAQUFBwIBFh1odHRwczovL3NlY3VyZS5jb21vZG8uY29t\nL0NQUzAIBgZngQwBAgEwVAYDVR0fBE0wSzBJoEegRYZDaHR0cDovL2NybC5jb21v\nZG9jYS5jb20vQ09NT0RPUlNBRG9tYWluVmFsaWRhdGlvblNlY3VyZVNlcnZlckNB\nLmNybDCBhQYIKwYBBQUHAQEEeTB3ME8GCCsGAQUFBzAChkNodHRwOi8vY3J0LmNv\nbW9kb2NhLmNvbS9DT01PRE9SU0FEb21haW5WYWxpZGF0aW9uU2VjdXJlU2VydmVy\nQ0EuY3J0MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5jb21vZG9jYS5jb20wKwYD\nVR0RBCQwIoIQKi5jb250ZW50ZnVsLmNvbYIOY29udGVudGZ1bC5jb20wDQYJKoZI\nhvcNAQELBQADggEBAGjTyCxabJc8vs4P2ayuF2/k8pr3oISpo8+bF4QFtXCQhr6I\n2G6OYvzZLWXCVFJ53FdT7PDIchP4tlYafySgXKo8POxfS20jBKk6+ZYEzwVlgRd2\njyhojQTNlilj9hPq3CJd4WK3KmA9Hnd9cRkdsduDcFeENviUWw/hgq3PvoYgGshh\nz9DzW878tMtAZk5DfiTkOvgphgvbaCod9W5MsDJ3NyQ6P88//28seyxs6yVTMKvM\nfsOz/kf3AgM+JbmAgpHZk8LkXI9qCIpjS9zcijRWz/QD8M/QedX5oKLB/PFBsP7k\n03x4tMWKes9Y3t+9Rdnx0kanOAkIzWLaZIh7igg=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIGCDCCA/CgAwIBAgIQKy5u6tl1NmwUim7bo3yMBzANBgkqhkiG9w0BAQwFADCB\nhTELMAkGA1UEBhMCR0IxGzAZBgNVBAgTEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4G\nA1UEBxMHU2FsZm9yZDEaMBgGA1UEChMRQ09NT0RPIENBIExpbWl0ZWQxKzApBgNV\nBAMTIkNPTU9ETyBSU0EgQ2VydGlmaWNhdGlvbiBBdXRob3JpdHkwHhcNMTQwMjEy\nMDAwMDAwWhcNMjkwMjExMjM1OTU5WjCBkDELMAkGA1UEBhMCR0IxGzAZBgNVBAgT\nEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4GA1UEBxMHU2FsZm9yZDEaMBgGA1UEChMR\nQ09NT0RPIENBIExpbWl0ZWQxNjA0BgNVBAMTLUNPTU9ETyBSU0EgRG9tYWluIFZh\nbGlkYXRpb24gU2VjdXJlIFNlcnZlciBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEP\nADCCAQoCggEBAI7CAhnhoFmk6zg1jSz9AdDTScBkxwtiBUUWOqigwAwCfx3M28Sh\nbXcDow+G+eMGnD4LgYqbSRutA776S9uMIO3Vzl5ljj4Nr0zCsLdFXlIvNN5IJGS0\nQa4Al/e+Z96e0HqnU4A7fK31llVvl0cKfIWLIpeNs4TgllfQcBhglo/uLQeTnaG6\nytHNe+nEKpooIZFNb5JPJaXyejXdJtxGpdCsWTWM/06RQ1A/WZMebFEh7lgUq/51\nUHg+TLAchhP6a5i84DuUHoVS3AOTJBhuyydRReZw3iVDpA3hSqXttn7IzW3uLh0n\nc13cRTCAquOyQQuvvUSH2rnlG51/ruWFgqUCAwEAAaOCAWUwggFhMB8GA1UdIwQY\nMBaAFLuvfgI9+qbxPISOre44mOzZMjLUMB0GA1UdDgQWBBSQr2o6lFoL2JDqElZz\n30O0Oija5zAOBgNVHQ8BAf8EBAMCAYYwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNV\nHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwGwYDVR0gBBQwEjAGBgRVHSAAMAgG\nBmeBDAECATBMBgNVHR8ERTBDMEGgP6A9hjtodHRwOi8vY3JsLmNvbW9kb2NhLmNv\nbS9DT01PRE9SU0FDZXJ0aWZpY2F0aW9uQXV0aG9yaXR5LmNybDBxBggrBgEFBQcB\nAQRlMGMwOwYIKwYBBQUHMAKGL2h0dHA6Ly9jcnQuY29tb2RvY2EuY29tL0NPTU9E\nT1JTQUFkZFRydXN0Q0EuY3J0MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5jb21v\nZG9jYS5jb20wDQYJKoZIhvcNAQEMBQADggIBAE4rdk+SHGI2ibp3wScF9BzWRJ2p\nmj6q1WZmAT7qSeaiNbz69t2Vjpk1mA42GHWx3d1Qcnyu3HeIzg/3kCDKo2cuH1Z/\ne+FE6kKVxF0NAVBGFfKBiVlsit2M8RKhjTpCipj4SzR7JzsItG8kO3KdY3RYPBps\nP0/HEZrIqPW1N+8QRcZs2eBelSaz662jue5/DJpmNXMyYE7l3YphLG5SEXdoltMY\ndVEVABt0iN3hxzgEQyjpFv3ZBdRdRydg1vs4O2xyopT4Qhrf7W8GjEXCBgCq5Ojc\n2bXhc3js9iPc0d1sjhqPpepUfJa3w/5Vjo1JXvxku88+vZbrac2/4EjxYoIQ5QxG\nV/Iz2tDIY+3GH5QFlkoakdH368+PUq4NCNk+qKBR6cGHdNXJ93SrLlP7u3r7l+L4\nHyaPs9Kg4DdbKDsx5Q5XLVq4rXmsXiBmGqW5prU5wfWYQ//u+aen/e7KJD2AFsQX\nj4rBYKEMrltDR5FL1ZoXX/nUh8HCjLfn4g8wGTeGrODcQgPmlKidrv0PJFGUzpII\n0fxQ8ANAe4hZ7Q7drNJ3gjTcBpUC2JD5Leo31Rpg0Gcg19hCC0Wvgmje3WYkN5Ap\nlBlGGSW4gNfL1IYoakRwJiNiqZ+Gb7+6kHDSVneFeO/qJakXzlByjAA6quPbYzSf\n+AZxAeKCINT+b72x\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIFdDCCBFygAwIBAgIQJ2buVutJ846r13Ci/ITeIjANBgkqhkiG9w0BAQwFADBv\nMQswCQYDVQQGEwJTRTEUMBIGA1UEChMLQWRkVHJ1c3QgQUIxJjAkBgNVBAsTHUFk\nZFRydXN0IEV4dGVybmFsIFRUUCBOZXR3b3JrMSIwIAYDVQQDExlBZGRUcnVzdCBF\neHRlcm5hbCBDQSBSb290MB4XDTAwMDUzMDEwNDgzOFoXDTIwMDUzMDEwNDgzOFow\ngYUxCzAJBgNVBAYTAkdCMRswGQYDVQQIExJHcmVhdGVyIE1hbmNoZXN0ZXIxEDAO\nBgNVBAcTB1NhbGZvcmQxGjAYBgNVBAoTEUNPTU9ETyBDQSBMaW1pdGVkMSswKQYD\nVQQDEyJDT01PRE8gUlNBIENlcnRpZmljYXRpb24gQXV0aG9yaXR5MIICIjANBgkq\nhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAkehUktIKVrGsDSTdxc9EZ3SZKzejfSNw\nAHG8U9/E+ioSj0t/EFa9n3Byt2F/yUsPF6c947AEYe7/EZfH9IY+Cvo+XPmT5jR6\n2RRr55yzhaCCenavcZDX7P0N+pxs+t+wgvQUfvm+xKYvT3+Zf7X8Z0NyvQwA1onr\nayzT7Y+YHBSrfuXjbvzYqOSSJNpDa2K4Vf3qwbxstovzDo2a5JtsaZn4eEgwRdWt\n4Q08RWD8MpZRJ7xnw8outmvqRsfHIKCxH2XeSAi6pE6p8oNGN4Tr6MyBSENnTnIq\nm1y9TBsoilwie7SrmNnu4FGDwwlGTm0+mfqVF9p8M1dBPI1R7Qu2XK8sYxrfV8g/\nvOldxJuvRZnio1oktLqpVj3Pb6r/SVi+8Kj/9Lit6Tf7urj0Czr56ENCHonYhMsT\n8dm74YlguIwoVqwUHZwK53Hrzw7dPamWoUi9PPevtQ0iTMARgexWO/bTouJbt7IE\nIlKVgJNp6I5MZfGRAy1wdALqi2cVKWlSArvX31BqVUa/oKMoYX9w0MOiqiwhqkfO\nKJwGRXa/ghgntNWutMtQ5mv0TIZxMOmm3xaG4Nj/QN370EKIf6MzOi5cHkERgWPO\nGHFrK+ymircxXDpqR+DDeVnWIBqv8mqYqnK8V0rSS527EPywTEHl7R09XiidnMy/\ns1Hap0flhFMCAwEAAaOB9DCB8TAfBgNVHSMEGDAWgBStvZh6NLQm9/rEJlTvA73g\nJMtUGjAdBgNVHQ4EFgQUu69+Aj36pvE8hI6t7jiY7NkyMtQwDgYDVR0PAQH/BAQD\nAgGGMA8GA1UdEwEB/wQFMAMBAf8wEQYDVR0gBAowCDAGBgRVHSAAMEQGA1UdHwQ9\nMDswOaA3oDWGM2h0dHA6Ly9jcmwudXNlcnRydXN0LmNvbS9BZGRUcnVzdEV4dGVy\nbmFsQ0FSb290LmNybDA1BggrBgEFBQcBAQQpMCcwJQYIKwYBBQUHMAGGGWh0dHA6\nLy9vY3NwLnVzZXJ0cnVzdC5jb20wDQYJKoZIhvcNAQEMBQADggEBAGS/g/FfmoXQ\nzbihKVcN6Fr30ek+8nYEbvFScLsePP9NDXRqzIGCJdPDoCpdTPW6i6FtxFQJdcfj\nJw5dhHk3QBN39bSsHNA7qxcS1u80GH4r6XnTq1dFDK8o+tDb5VCViLvfhVdpfZLY\nUspzgb8c8+a4bmYRBbMelC1/kZWSWfFMzqORcUx8Rww7Cxn2obFshj5cqsQugsv5\nB5a6SE2Q8pTIqXOi6wZ7I53eovNNVZ96YUWYGGjHXkBrI/V5eu+MtWuLt29G9Hvx\nPUsE2JOAWVrgQSQdso8VYFhH2+9uRv0V9dlfmrPb2LjkQLPNlzmuhbsdjrzch5vR\npu/xO28QOG8=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIENjCCAx6gAwIBAgIBATANBgkqhkiG9w0BAQUFADBvMQswCQYDVQQGEwJTRTEU\nMBIGA1UEChMLQWRkVHJ1c3QgQUIxJjAkBgNVBAsTHUFkZFRydXN0IEV4dGVybmFs\nIFRUUCBOZXR3b3JrMSIwIAYDVQQDExlBZGRUcnVzdCBFeHRlcm5hbCBDQSBSb290\nMB4XDTAwMDUzMDEwNDgzOFoXDTIwMDUzMDEwNDgzOFowbzELMAkGA1UEBhMCU0Ux\nFDASBgNVBAoTC0FkZFRydXN0IEFCMSYwJAYDVQQLEx1BZGRUcnVzdCBFeHRlcm5h\nbCBUVFAgTmV0d29yazEiMCAGA1UEAxMZQWRkVHJ1c3QgRXh0ZXJuYWwgQ0EgUm9v\ndDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALf3GjPm8gAELTngTlvt\nH7xsD821+iO2zt6bETOXpClMfZOfvUq8k+0DGuOPz+VtUFrWlymUWoCwSXrbLpX9\nuMq/NzgtHj6RQa1wVsfwTz/oMp50ysiQVOnGXw94nZpAPA6sYapeFI+eh6FqUNzX\nmk6vBbOmcZSccbNQYArHE504B4YCqOmoaSYYkKtMsE8jqzpPhNjfzp/haW+710LX\na0Tkx63ubUFfclpxCDezeWWkWaCUN/cALw3CknLa0Dhy2xSoRcRdKn23tNbE7qzN\nE0S3ySvdQwAl+mG5aWpYIxG3pzOPVnVZ9c0p10a3CitlttNCbxWyuHv77+ldU9U0\nWicCAwEAAaOB3DCB2TAdBgNVHQ4EFgQUrb2YejS0Jvf6xCZU7wO94CTLVBowCwYD\nVR0PBAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wgZkGA1UdIwSBkTCBjoAUrb2YejS0\nJvf6xCZU7wO94CTLVBqhc6RxMG8xCzAJBgNVBAYTAlNFMRQwEgYDVQQKEwtBZGRU\ncnVzdCBBQjEmMCQGA1UECxMdQWRkVHJ1c3QgRXh0ZXJuYWwgVFRQIE5ldHdvcmsx\nIjAgBgNVBAMTGUFkZFRydXN0IEV4dGVybmFsIENBIFJvb3SCAQEwDQYJKoZIhvcN\nAQEFBQADggEBALCb4IUlwtYj4g+WBpKdQZic2YR5gdkeWxQHIzZlj7DYd7usQWxH\nYINRsPkyPef89iYTx4AWpb9a/IfPeHmJIZriTAcKhjW88t5RxNKWt9x+Tu5w/Rw5\n6wwCURQtjr0W4MHfRnXnJK3s9EK0hZNwEGe6nQY1ShjTK3rMUUKhemPR5ruhxSvC\nNr4TDea9Y355e6cJDUCrat2PisP29owaQgVR1EX1n6diIWgVIEM8med8vSTYqZEX\nc4g/VhsxOBi0cQ+azcgOno4uG+GMmIPLHzHxREzGBHNJdmAPx/i9F4BrLunMTA5a\nmnkPIAou1Z5jJh5VkpTYghdae9C8x49OhgQ=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIKuzCCCaOgAwIBAgIMNjYXNLLXqLY/TOgcMA0GCSqGSIb3DQEBCwUAMFcxCzAJ\nBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9iYWxTaWduIG52LXNhMS0wKwYDVQQDEyRH\nbG9iYWxTaWduIENsb3VkU1NMIENBIC0gU0hBMjU2IC0gRzMwHhcNMTcwMzIzMTEy\nMzE5WhcNMTcxMTAzMTEyMjIzWjBgMQswCQYDVQQGEwJVUzERMA8GA1UECBMIRGVs\nYXdhcmUxDjAMBgNVBAcTBURvdmVyMRYwFAYDVQQKEw1JbmNhcHN1bGEgSW5jMRYw\nFAYDVQQDEw1pbmNhcHN1bGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB\nCgKCAQEA3sgLKGR+l4KI2c5rjhwTQcuXu32CXy2BuUqZkco4pDsHzvBWKNAzxqSe\nUOt4LmhBzLETdHqvsXVPnNW4mbruVbreTZoJ6//Jy4WfBAtvVq/uL9krmv19opt1\n0Ll4+LLpeI+VBnkCNBPrALk/7aoVHeZcTmRmq2QPbRIBSaa+ZQhL+swQc+ATmcxd\nz9ew1gZPo9kWC4lZ63FIRu97wa3yKWi1qD8alv1EKFgInZOLtp359BOJlLvTt4zC\nnAzvMZog8YXJ9Mit5mSWTBGfKp0Z4QF5GIQNcM2rPFZC3nWjwjp7DwgsCvpJDzHR\nkpULclhrBX7LvbvMdMzT2KgvvrwZtwIDAQABo4IHfDCCB3gwDgYDVR0PAQH/BAQD\nAgWgMIGKBggrBgEFBQcBAQR+MHwwQgYIKwYBBQUHMAKGNmh0dHA6Ly9zZWN1cmUu\nZ2xvYmFsc2lnbi5jb20vY2FjZXJ0L2Nsb3Vkc3Nsc2hhMmczLmNydDA2BggrBgEF\nBQcwAYYqaHR0cDovL29jc3AyLmdsb2JhbHNpZ24uY29tL2Nsb3Vkc3Nsc2hhMmcz\nMFYGA1UdIARPME0wQQYJKwYBBAGgMgEUMDQwMgYIKwYBBQUHAgEWJmh0dHBzOi8v\nd3d3Lmdsb2JhbHNpZ24uY29tL3JlcG9zaXRvcnkvMAgGBmeBDAECAjAJBgNVHRME\nAjAAMIIGFQYDVR0RBIIGDDCCBgiCDWluY2Fwc3VsYS5jb22CHSouYWNjZXB0YXRp\nZS1lbmdpZS1lbmVyZ2llLm5sgg0qLmFtd2FsYWsuY29tghsqLmFwcGx5LmdvbWFz\ndGVyY2FyZC5jb20uYXWCCyouYXZpdmEuY29tghEqLmF2aXZhY2FuYWRhLmNvbYIQ\nKi5iaW5nb21hbmlhLmNvbYIRKi5icmFuY2hldHZvdXMuZnKCEyouY2xlYXJza3kt\nZGF0YS5uZXSCDiouY29uZWN0eXMuY29tghAqLmNvbnRlbnRmdWwuY29tgg4qLmNv\ncmVmb3VyLmNvbYIMKi5jb3JzYWlyLmNpggwqLmNvcnNhaXIuZ3CCDCouY29yc2Fp\nci5tcYIMKi5jb3JzYWlyLnNughQqLmNyZWRpdG95Y2F1Y2lvbi5lc4IWKi5kYzQu\ncGFnZXVwcGVvcGxlLmNvbYIWKi5kZXZpY2Vwcm90ZWN0aW9uLmNvbYISKi5kaWFq\ndWdvc28xMjMuY29tghUqLmRpcmVjdG1vYmlsZXMuY28udWuCEyouZWRkaWVhbmRj\nby5jb20uYXWCFCouZWtlZXBlcmdyb3VwLmNvLnVrggwqLmVwaWR1by5jb22CESou\nZXBpZHVvZm9ydGUuY29tghUqLmV2b3F1YWFkdmFudGFnZS5jb22CFSouaGVkZ2Vz\ndG9uZWdyb3VwLmNvbYIUKi5pbmNhcHN1bGEtZGVtby5iaXqCDCouaXZyYXBwLm5l\ndIINKi5rcC1teXBnLmNvbYIYKi5sYi5uZXN0bGUtd2F0ZXJzbmEuY29tgg0qLmx1\nbHVjcm0uY29tghMqLm1hZGV3aXRobmVzdGxlLmNhgg4qLm1hcmljb3BhLmVkdYIU\nKi5tYXR0ZWxwYXJ0bmVycy5jb22CGioubXhwLnphbXNoLmluY2Fwc3VsYS5tb2Jp\ngg0qLm15ZXIuY29tLmF1gg0qLm15bnZhcHAuY29tgg8qLm5ldDJwaG9uZS5jb22C\nDSoucGluMTExMS5jb22CECoucHJvZC5pbG9hbi5jb22CGCouc2Nhbm5lci5zcG90\nb3B0aW9uLmNvbYIZKi5zZWFyY2hmbG93c3RhZ2luZy5jby51a4IXKi5zaW1wbHli\nZXR0ZXJ0aW4uY28udWuCCiouc29mbi5jb22CDiouc3RyYXR0b24uY29tghcqLnRl\nc3QtZW5naWUtZW5lcmdpZS5ubIIQKi50cmF2ZWxwb3J0LmNvbYIOKi50cmVtYmxh\nbnQuY2GCCyoudml0dGVsLmpwgg0qLnZ0ZWNoLmNvLnVrgg0qLndlcmFsbHkuY29t\nghcqLndoaXRlaG91c2VoaXN0b3J5Lm9yZ4IMKi53cnBzLm9uLmNhggkqLnd0ZS5u\nZXSCCyoueW91ZmkuY29tghthY2NlcHRhdGllLWVuZ2llLWVuZXJnaWUubmyCC2Ft\nd2FsYWsuY29tgglhdml2YS5jb22CDmJpbmdvbWFuaWEuY29tgg9icmFuY2hldHZv\ndXMuZnKCDGNvbmVjdHlzLmNvbYIKY29yc2Fpci5jaYIKY29yc2Fpci5ncIIKY29y\nc2Fpci5tcYIKY29yc2Fpci5zboIUZGV2aWNlcHJvdGVjdGlvbi5jb22CEGRpYWp1\nZ29zbzEyMy5jb22CE2RpcmVjdG1vYmlsZXMuY28udWuCEWVkZGllYW5kY28uY29t\nLmF1ggplcGlkdW8uY29tgg9lcGlkdW9mb3J0ZS5jb22CE2V2b3F1YWFkdmFudGFn\nZS5jb22CE2hlZGdlc3RvbmVncm91cC5jb22CC2twLW15cGcuY29tghFtYWRld2l0\naG5lc3RsZS5jYYINbmV0MnBob25lLmNvbYILcGluMTExMS5jb22CF3NlYXJjaGZs\nb3dzdGFnaW5nLmNvLnVrghVzaW1wbHliZXR0ZXJ0aW4uY28udWuCFXRlc3QtZW5n\naWUtZW5lcmdpZS5ubIIOdHJhdmVscG9ydC5jb22CC3Z0ZWNoLmNvLnVrggt3ZXJh\nbGx5LmNvbYIKd3Jwcy5vbi5jYYIHd3RlLm5ldDAdBgNVHSUEFjAUBggrBgEFBQcD\nAQYIKwYBBQUHAwIwHQYDVR0OBBYEFFAxG/f60PL3CFmmnW/Uk9gBtDhCMB8GA1Ud\nIwQYMBaAFKkrh+HOJEc7G7/PhTcCVZ0NlFjmMA0GCSqGSIb3DQEBCwUAA4IBAQAn\n/+V5a6hfMgXZJa5H0c0Gu5E3bSl8gvQsS4VsT1tnI0OnjwqQtd4fRC2TQFogSYbk\nDfmtFxiiQymF9CtlRbTQX41gZ6RMLRIyeA96k7WC3PJiIlqiFDp0172INTU0NsiX\nfJ/u2plLINtye67yUt38TGOYZa1aF/mjAtN+tubumY1Va7k/ec4b+qhZQUOzgGZv\nTrx5wDYy8UBeUkqn7/ZVH7FDAN6wcc97e49/02okiH7pu9bSlP9Izl6YSaLBRcAy\nTAixepqGxVjhK1OmKMIGrIiI2H9oEelpNImMvgQo8XK+2Bcvnw/H5qQg8VmFwPd7\ndfeszhlA0vV4mgGQMm5T\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIEizCCA3OgAwIBAgIORvCM288sVGbvMwHdXzQwDQYJKoZIhvcNAQELBQAwVzEL\nMAkGA1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsT\nB1Jvb3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw0xNTA4MTkw\nMDAwMDBaFw0yNTA4MTkwMDAwMDBaMFcxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBH\nbG9iYWxTaWduIG52LXNhMS0wKwYDVQQDEyRHbG9iYWxTaWduIENsb3VkU1NMIENB\nIC0gU0hBMjU2IC0gRzMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCj\nwHXhMpjl2a6EfI3oI19GlVtMoiVw15AEhYDJtfSKZU2Sy6XEQqC2eSUx7fGFIM0T\nUT1nrJdNaJszhlyzey2q33egYdH1PPua/NPVlMrJHoAbkJDIrI32YBecMbjFYaLi\nblclCG8kmZnPlL/Hi2uwH8oU+hibbBB8mSvaSmPlsk7C/T4QC0j0dwsv8JZLOu69\nNd6FjdoTDs4BxHHT03fFCKZgOSWnJ2lcg9FvdnjuxURbRb0pO+LGCQ+ivivc41za\nWm+O58kHa36hwFOVgongeFxyqGy+Z2ur5zPZh/L4XCf09io7h+/awkfav6zrJ2R7\nTFPrNOEvmyBNVBJrfSi9AgMBAAGjggFTMIIBTzAOBgNVHQ8BAf8EBAMCAQYwHQYD\nVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMBIGA1UdEwEB/wQIMAYBAf8CAQAw\nHQYDVR0OBBYEFKkrh+HOJEc7G7/PhTcCVZ0NlFjmMB8GA1UdIwQYMBaAFGB7ZhpF\nDZfKiVAvfQTNNKj//P1LMD0GCCsGAQUFBwEBBDEwLzAtBggrBgEFBQcwAYYhaHR0\ncDovL29jc3AuZ2xvYmFsc2lnbi5jb20vcm9vdHIxMDMGA1UdHwQsMCowKKAmoCSG\nImh0dHA6Ly9jcmwuZ2xvYmFsc2lnbi5jb20vcm9vdC5jcmwwVgYDVR0gBE8wTTAL\nBgkrBgEEAaAyARQwPgYGZ4EMAQICMDQwMgYIKwYBBQUHAgEWJmh0dHBzOi8vd3d3\nLmdsb2JhbHNpZ24uY29tL3JlcG9zaXRvcnkvMA0GCSqGSIb3DQEBCwUAA4IBAQCi\nHWmKCo7EFIMqKhJNOSeQTvCNrNKWYkc2XpLR+sWTtTcHZSnS9FNQa8n0/jT13bgd\n+vzcFKxWlCecQqoETbftWNmZ0knmIC/Tp3e4Koka76fPhi3WU+kLk5xOq9lF7qSE\nhf805A7Au6XOX5WJhXCqwV3szyvT2YPfA8qBpwIyt3dhECVO2XTz2XmCtSZwtFK8\njzPXiq4Z0PySrS+6PKBIWEde/SBWlSDBch2rZpmk1Xg3SBufskw3Z3r9QtLTVp7T\nHY7EDGiWtkdREPd76xUJZPX58GMWLT3fI0I6k2PMq69PVwbH/hRVYs4nERnh9ELt\nIjBrNRpKBYCkZd/My2/Q\n-----END CERTIFICATE-----\n"))
	return &ContentClient{
		Locale:    locale,
		authToken: authToken,
		client:    &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}},
		host:      fmt.Sprintf("https://%s", contentfulCPAURL),
		pool:      pool,
		spaceID:   "ygx37epqlss8",
	}
}

// ManagementClient implements a space specific contentful client
type ManagementClient struct {
	host      string
	spaceID   string
	authToken string
	client    *http.Client
	pool      *x509.CertPool
}

// NewManagement returns a contentful client interfacing with the content management api
func NewManagement(authToken string) *ManagementClient {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte("-----BEGIN CERTIFICATE-----\nMIIL6TCCCtGgAwIBAgIQBigdNnW0H8yz/xj67Pj93zANBgkqhkiG9w0BAQsFADBw\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3\nd3cuZGlnaWNlcnQuY29tMS8wLQYDVQQDEyZEaWdpQ2VydCBTSEEyIEhpZ2ggQXNz\ndXJhbmNlIFNlcnZlciBDQTAeFw0xNDEyMDgwMDAwMDBaFw0xODAyMDYxMjAwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRUwEwYDVQQKEwxGYXN0bHksIEluYy4xGTAXBgNVBAMTEGEu\nc3NsLmZhc3RseS5uZXQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDU\nJUiQsaVP/vC4Mb3aJUmA9KnMQa7EJfjYLsE4F0VehrOp8jlSSXmQLELlUAwPp2F2\nPNyB32DDOFBHZIYwFrApFEzsJdTKQUYk6xHPZOdYoIijpmfb5xRMdTjqxThGkk+k\nhU0+ipPWiErJNRkapLgPwPD4ctd5X8rnKF8lMHIxx5Xhdg6PqZC3F7y45Nym2a3M\n8xIKIkB77o1bkuDpGnV9ZESC/Yf9Mc4NmWrQjqQc+8yIabir+n7/YcM5UdUjZPNS\nhgL4jLYVJ+KDRZcjIT/dXRZoPpJgRFL9NIep/eSAzQa3g659uW7tjN6tg5iQm4hw\nksaWp+zfTAJc4IXNtlndAgMBAAGjggiBMIIIfTAfBgNVHSMEGDAWgBRRaP+QrwIH\ndTzM2WVkYqISuFlyOzAdBgNVHQ4EFgQUwIj0Y03ka1Q28RLCtKWy4nN7FIgwggax\nBgNVHREEggaoMIIGpIIQYS5zc2wuZmFzdGx5Lm5ldIISKi5hLnNzbC5mYXN0bHku\nbmV0gg9mYXN0Lndpc3RpYS5jb22CEHB1cmdlLmZhc3RseS5uZXSCEm1pcnJvcnMu\nZmFzdGx5Lm5ldIIOKi5wYXJzZWNkbi5jb22CDSouZmFzdHNzbC5uZXSCCXZveGVy\nLmNvbYINd3d3LnZveGVyLmNvbYIOKi5maXJlYmFzZS5jb22CEHNpdGVzLnlhbW1l\nci5jb22CGHNpdGVzLnN0YWdpbmcueWFtbWVyLmNvbYIPKi5za2ltbGlua3MuY29t\nghMqLnNraW1yZXNvdXJjZXMuY29tghBjZG4udGhpbmdsaW5rLm1lggwqLmZpdGJp\ndC5jb22CEiouaG9zdHMuZmFzdGx5Lm5ldIISY29udHJvbC5mYXN0bHkubmV0gg8q\nLndpa2lhLWluYy5jb22CFSoucGVyZmVjdGF1ZGllbmNlLmNvbYILKi53aWtpYS5j\nb22CEmYuY2xvdWQuZ2l0aHViLmNvbYIVKi5kaWdpdGFsc2Npcm9jY28ubmV0ggoq\nLmV0c3kuY29tghAqLmV0c3lzdGF0aWMuY29tgg0qLmFkZHRoaXMuY29tghAqLmFk\nZHRoaXNjZG4uY29tgg9mYXN0Lndpc3RpYS5uZXSCDnJhdy5naXRodWIuY29tgg93\nd3cudXNlcmZveC5jb22CEyouYXNzZXRzLXlhbW1lci5jb22CGyouc3RhZ2luZy5h\nc3NldHMteWFtbWVyLmNvbYIWYXNzZXRzLmh1Z2dpZXMtY2RuLm5ldIISb3JiaXQu\nc2hhemFtaWQuY29tgg9hYm91dC5qc3Rvci5vcmeCFyouZ2xvYmFsLnNzbC5mYXN0\nbHkubmV0gg13ZWIudm94ZXIuY29tgg9weXBpLnB5dGhvbi5vcmeCCyouMTJ3YnQu\nY29tghJ3d3cuaG9sZGVyZGVvcmQubm+CGnNlY3VyZWQuaW5kbi5pbmZvbGlua3Mu\nY29tghBwbGF5LnZpZHlhcmQuY29tghhwbGF5LXN0YWdpbmcudmlkeWFyZC5jb22C\nFXNlY3VyZS5pbWcud2ZyY2RuLmNvbYIWc2VjdXJlLmltZy5qb3NzY2RuLmNvbYIQ\nKi5nb2NhcmRsZXNzLmNvbYIVd2lkZ2V0cy5waW50ZXJlc3QuY29tgg4qLjdkaWdp\ndGFsLmNvbYINKi43c3RhdGljLmNvbYIPcC5kYXRhZG9naHEuY29tghBuZXcubXVs\nYmVycnkuY29tghJ3d3cuc2FmYXJpZmxvdy5jb22CEmNkbi5jb250ZW50ZnVsLmNv\nbYIQdG9vbHMuZmFzdGx5Lm5ldIISKi5odWV2b3NidWVub3MuY29tgg4qLmdvb2Rl\nZ2dzLmNvbYIWKi5mYXN0bHkucGljbW9ua2V5LmNvbYIVKi5jZG4ud2hpcHBsZWhp\nbGwubmV0ghEqLndoaXBwbGVoaWxsLm5ldIIbY2RuLm1lZGlhMzQud2hpcHBsZWhp\nbGwubmV0ghtjZG4ubWVkaWE1Ni53aGlwcGxlaGlsbC5uZXSCG2Nkbi5tZWRpYTc4\nLndoaXBwbGVoaWxsLm5ldIIcY2RuLm1lZGlhOTEwLndoaXBwbGVoaWxsLm5ldIIO\nKi5tb2RjbG90aC5jb22CDyouZGlzcXVzY2RuLmNvbYILKi5qc3Rvci5vcmeCDyou\nZHJlYW1ob3N0LmNvbYIOd3d3LmZsaW50by5jb22CDyouY2hhcnRiZWF0LmNvbYIN\nKi5oaXBtdW5rLmNvbYIaY29udGVudC5iZWF2ZXJicm9va3MuY28udWuCG3NlY3Vy\nZS5jb21tb24uY3Nuc3RvcmVzLmNvbYIOd3d3LmpvaW5vcy5jb22CJXN0YWdpbmct\nbW9iaWxlLWNvbGxlY3Rvci5uZXdyZWxpYy5jb22CDioubW9kY2xvdGgubmV0ghAq\nLmZvdXJzcXVhcmUuY29tggwqLnNoYXphbS5jb22CCiouNHNxaS5uZXSCDioubWV0\nYWNwYW4ub3JnggwqLmZhc3RseS5jb22CCXdpa2lhLmNvbYIKZmFzdGx5LmNvbYIR\nKi5nYWR2ZW50dXJlcy5jb22CFnd3dy5nYWR2ZW50dXJlcy5jb20uYXWCFXd3dy5n\nYWR2ZW50dXJlcy5jby51a4IJa3JlZG8uY29tghZjZG4tdGFncy5icmFpbmllbnQu\nY29tghRteS5iaWxsc3ByaW5nYXBwLmNvbYIGcnZtLmlvMA4GA1UdDwEB/wQEAwIF\noDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwdQYDVR0fBG4wbDA0oDKg\nMIYuaHR0cDovL2NybDMuZGlnaWNlcnQuY29tL3NoYTItaGEtc2VydmVyLWc1LmNy\nbDA0oDKgMIYuaHR0cDovL2NybDQuZGlnaWNlcnQuY29tL3NoYTItaGEtc2VydmVy\nLWc1LmNybDBMBgNVHSAERTBDMDcGCWCGSAGG/WwBATAqMCgGCCsGAQUFBwIBFhxo\ndHRwczovL3d3dy5kaWdpY2VydC5jb20vQ1BTMAgGBmeBDAECAjCBgwYIKwYBBQUH\nAQEEdzB1MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2VydC5jb20wTQYI\nKwYBBQUHMAKGQWh0dHA6Ly9jYWNlcnRzLmRpZ2ljZXJ0LmNvbS9EaWdpQ2VydFNI\nQTJIaWdoQXNzdXJhbmNlU2VydmVyQ0EuY3J0MAwGA1UdEwEB/wQCMAAwDQYJKoZI\nhvcNAQELBQADggEBAKLWzbX7wSyjzE7BVMjLrHAaiz+WGSwrAPrQBJ29sqouu9gv\nI7i2Ie6eiRb4YLMouy6D+ZNZ+RM+Hkjv+PZFxCcDRmaWi+74ha5d8O155gRJRPZ0\nSy5SfD/8kqrJRfC+/D/KdQzOroD4sx6Qprs9lZ0IEn4CTf0YPNV+Cps37LsVyPJL\nfjDlGIM5K3B/vtZfn2f8buQ9QyKiN0bc67GdCjih9dSrkQNkxJiEOwqiSjYtkdFO\ndYpXF8d1rQKV7a6z2vJloDwilfXLLlUX7rA3qVu7r4EUfIsZgH7hgB4bbst7tx+7\nPgUEq2334kKPVFpsxgsj5++k4lh7tNlakXiBUtw=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIEsTCCA5mgAwIBAgIQBOHnpNxc8vNtwCtCuF0VnzANBgkqhkiG9w0BAQsFADBs\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3\nd3cuZGlnaWNlcnQuY29tMSswKQYDVQQDEyJEaWdpQ2VydCBIaWdoIEFzc3VyYW5j\nZSBFViBSb290IENBMB4XDTEzMTAyMjEyMDAwMFoXDTI4MTAyMjEyMDAwMFowcDEL\nMAkGA1UEBhMCVVMxFTATBgNVBAoTDERpZ2lDZXJ0IEluYzEZMBcGA1UECxMQd3d3\nLmRpZ2ljZXJ0LmNvbTEvMC0GA1UEAxMmRGlnaUNlcnQgU0hBMiBIaWdoIEFzc3Vy\nYW5jZSBTZXJ2ZXIgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC2\n4C/CJAbIbQRf1+8KZAayfSImZRauQkCbztyfn3YHPsMwVYcZuU+UDlqUH1VWtMIC\nKq/QmO4LQNfE0DtyyBSe75CxEamu0si4QzrZCwvV1ZX1QK/IHe1NnF9Xt4ZQaJn1\nitrSxwUfqJfJ3KSxgoQtxq2lnMcZgqaFD15EWCo3j/018QsIJzJa9buLnqS9UdAn\n4t07QjOjBSjEuyjMmqwrIw14xnvmXnG3Sj4I+4G3FhahnSMSTeXXkgisdaScus0X\nsh5ENWV/UyU50RwKmmMbGZJ0aAo3wsJSSMs5WqK24V3B3aAguCGikyZvFEohQcft\nbZvySC/zA/WiaJJTL17jAgMBAAGjggFJMIIBRTASBgNVHRMBAf8ECDAGAQH/AgEA\nMA4GA1UdDwEB/wQEAwIBhjAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIw\nNAYIKwYBBQUHAQEEKDAmMCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2Vy\ndC5jb20wSwYDVR0fBEQwQjBAoD6gPIY6aHR0cDovL2NybDQuZGlnaWNlcnQuY29t\nL0RpZ2lDZXJ0SGlnaEFzc3VyYW5jZUVWUm9vdENBLmNybDA9BgNVHSAENjA0MDIG\nBFUdIAAwKjAoBggrBgEFBQcCARYcaHR0cHM6Ly93d3cuZGlnaWNlcnQuY29tL0NQ\nUzAdBgNVHQ4EFgQUUWj/kK8CB3U8zNllZGKiErhZcjswHwYDVR0jBBgwFoAUsT7D\naQP4v0cB1JgmGggC72NkK8MwDQYJKoZIhvcNAQELBQADggEBABiKlYkD5m3fXPwd\naOpKj4PWUS+Na0QWnqxj9dJubISZi6qBcYRb7TROsLd5kinMLYBq8I4g4Xmk/gNH\nE+r1hspZcX30BJZr01lYPf7TMSVcGDiEo+afgv2MW5gxTs14nhr9hctJqvIni5ly\n/D6q1UEL2tU2ob8cbkdJf17ZSHwD2f2LSaCYJkJA69aSEaRkCldUxPUd1gJea6zu\nxICaEnL6VpPX/78whQYwvwt/Tv9XBZ0k7YXDK/umdaisLRbvfXknsuvCnQsH6qqF\n0wGjIChBWUMo0oHjqvbsezt3tkBigAVBRQHvFwY+3sAzm2fTYS5yh+Rp/BIAV0Ae\ncPUeybQ=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIFWTCCBEGgAwIBAgIRAI2kTzBRQqhJ3nOm/ZZpYacwDQYJKoZIhvcNAQELBQAw\ngZAxCzAJBgNVBAYTAkdCMRswGQYDVQQIExJHcmVhdGVyIE1hbmNoZXN0ZXIxEDAO\nBgNVBAcTB1NhbGZvcmQxGjAYBgNVBAoTEUNPTU9ETyBDQSBMaW1pdGVkMTYwNAYD\nVQQDEy1DT01PRE8gUlNBIERvbWFpbiBWYWxpZGF0aW9uIFNlY3VyZSBTZXJ2ZXIg\nQ0EwHhcNMTYwNjI3MDAwMDAwWhcNMTcwNzI3MjM1OTU5WjBeMSEwHwYDVQQLExhE\nb21haW4gQ29udHJvbCBWYWxpZGF0ZWQxHjAcBgNVBAsTFUVzc2VudGlhbFNTTCBX\naWxkY2FyZDEZMBcGA1UEAwwQKi5jb250ZW50ZnVsLmNvbTCCASIwDQYJKoZIhvcN\nAQEBBQADggEPADCCAQoCggEBALCfMS7doJgi6LkkMuNxGyurtC8Vcm0GtOcWZuf3\nCwauhbwQSHIVxJ8ggcnoNmVXXJN1hqctFUpapt2JLAuwUQUc/k6QJY8M06nWytJI\np3Lf6o3bkWMBxbbIGV6L1ybmtBnh2lRCIw1MSnD620tEAH1om2UIgIPPI/6fH4ZC\n8P7S4/2ImJ9EsbGUuYoBPIP2pIcNMP+lRaIGpPqyffyP46Tr0gAhPC8SOctfRCRe\n5DjPkWTFCIK/X7wux5VWEKhk+ZmpN/E/930ixwZynNqGr/7GVWh4Vvqc7GgNb1yO\nl3co4xwSbdseCYL2eWWDrisP+h7KygIGKpZ116wjUjjClk0CAwEAAaOCAd0wggHZ\nMB8GA1UdIwQYMBaAFJCvajqUWgvYkOoSVnPfQ7Q6KNrnMB0GA1UdDgQWBBRSLKYh\nJNKm4ApiHS0i4VLS8TvSDzAOBgNVHQ8BAf8EBAMCBaAwDAYDVR0TAQH/BAIwADAd\nBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwTwYDVR0gBEgwRjA6BgsrBgEE\nAbIxAQICBzArMCkGCCsGAQUFBwIBFh1odHRwczovL3NlY3VyZS5jb21vZG8uY29t\nL0NQUzAIBgZngQwBAgEwVAYDVR0fBE0wSzBJoEegRYZDaHR0cDovL2NybC5jb21v\nZG9jYS5jb20vQ09NT0RPUlNBRG9tYWluVmFsaWRhdGlvblNlY3VyZVNlcnZlckNB\nLmNybDCBhQYIKwYBBQUHAQEEeTB3ME8GCCsGAQUFBzAChkNodHRwOi8vY3J0LmNv\nbW9kb2NhLmNvbS9DT01PRE9SU0FEb21haW5WYWxpZGF0aW9uU2VjdXJlU2VydmVy\nQ0EuY3J0MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5jb21vZG9jYS5jb20wKwYD\nVR0RBCQwIoIQKi5jb250ZW50ZnVsLmNvbYIOY29udGVudGZ1bC5jb20wDQYJKoZI\nhvcNAQELBQADggEBAGjTyCxabJc8vs4P2ayuF2/k8pr3oISpo8+bF4QFtXCQhr6I\n2G6OYvzZLWXCVFJ53FdT7PDIchP4tlYafySgXKo8POxfS20jBKk6+ZYEzwVlgRd2\njyhojQTNlilj9hPq3CJd4WK3KmA9Hnd9cRkdsduDcFeENviUWw/hgq3PvoYgGshh\nz9DzW878tMtAZk5DfiTkOvgphgvbaCod9W5MsDJ3NyQ6P88//28seyxs6yVTMKvM\nfsOz/kf3AgM+JbmAgpHZk8LkXI9qCIpjS9zcijRWz/QD8M/QedX5oKLB/PFBsP7k\n03x4tMWKes9Y3t+9Rdnx0kanOAkIzWLaZIh7igg=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIGCDCCA/CgAwIBAgIQKy5u6tl1NmwUim7bo3yMBzANBgkqhkiG9w0BAQwFADCB\nhTELMAkGA1UEBhMCR0IxGzAZBgNVBAgTEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4G\nA1UEBxMHU2FsZm9yZDEaMBgGA1UEChMRQ09NT0RPIENBIExpbWl0ZWQxKzApBgNV\nBAMTIkNPTU9ETyBSU0EgQ2VydGlmaWNhdGlvbiBBdXRob3JpdHkwHhcNMTQwMjEy\nMDAwMDAwWhcNMjkwMjExMjM1OTU5WjCBkDELMAkGA1UEBhMCR0IxGzAZBgNVBAgT\nEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4GA1UEBxMHU2FsZm9yZDEaMBgGA1UEChMR\nQ09NT0RPIENBIExpbWl0ZWQxNjA0BgNVBAMTLUNPTU9ETyBSU0EgRG9tYWluIFZh\nbGlkYXRpb24gU2VjdXJlIFNlcnZlciBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEP\nADCCAQoCggEBAI7CAhnhoFmk6zg1jSz9AdDTScBkxwtiBUUWOqigwAwCfx3M28Sh\nbXcDow+G+eMGnD4LgYqbSRutA776S9uMIO3Vzl5ljj4Nr0zCsLdFXlIvNN5IJGS0\nQa4Al/e+Z96e0HqnU4A7fK31llVvl0cKfIWLIpeNs4TgllfQcBhglo/uLQeTnaG6\nytHNe+nEKpooIZFNb5JPJaXyejXdJtxGpdCsWTWM/06RQ1A/WZMebFEh7lgUq/51\nUHg+TLAchhP6a5i84DuUHoVS3AOTJBhuyydRReZw3iVDpA3hSqXttn7IzW3uLh0n\nc13cRTCAquOyQQuvvUSH2rnlG51/ruWFgqUCAwEAAaOCAWUwggFhMB8GA1UdIwQY\nMBaAFLuvfgI9+qbxPISOre44mOzZMjLUMB0GA1UdDgQWBBSQr2o6lFoL2JDqElZz\n30O0Oija5zAOBgNVHQ8BAf8EBAMCAYYwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNV\nHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwGwYDVR0gBBQwEjAGBgRVHSAAMAgG\nBmeBDAECATBMBgNVHR8ERTBDMEGgP6A9hjtodHRwOi8vY3JsLmNvbW9kb2NhLmNv\nbS9DT01PRE9SU0FDZXJ0aWZpY2F0aW9uQXV0aG9yaXR5LmNybDBxBggrBgEFBQcB\nAQRlMGMwOwYIKwYBBQUHMAKGL2h0dHA6Ly9jcnQuY29tb2RvY2EuY29tL0NPTU9E\nT1JTQUFkZFRydXN0Q0EuY3J0MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5jb21v\nZG9jYS5jb20wDQYJKoZIhvcNAQEMBQADggIBAE4rdk+SHGI2ibp3wScF9BzWRJ2p\nmj6q1WZmAT7qSeaiNbz69t2Vjpk1mA42GHWx3d1Qcnyu3HeIzg/3kCDKo2cuH1Z/\ne+FE6kKVxF0NAVBGFfKBiVlsit2M8RKhjTpCipj4SzR7JzsItG8kO3KdY3RYPBps\nP0/HEZrIqPW1N+8QRcZs2eBelSaz662jue5/DJpmNXMyYE7l3YphLG5SEXdoltMY\ndVEVABt0iN3hxzgEQyjpFv3ZBdRdRydg1vs4O2xyopT4Qhrf7W8GjEXCBgCq5Ojc\n2bXhc3js9iPc0d1sjhqPpepUfJa3w/5Vjo1JXvxku88+vZbrac2/4EjxYoIQ5QxG\nV/Iz2tDIY+3GH5QFlkoakdH368+PUq4NCNk+qKBR6cGHdNXJ93SrLlP7u3r7l+L4\nHyaPs9Kg4DdbKDsx5Q5XLVq4rXmsXiBmGqW5prU5wfWYQ//u+aen/e7KJD2AFsQX\nj4rBYKEMrltDR5FL1ZoXX/nUh8HCjLfn4g8wGTeGrODcQgPmlKidrv0PJFGUzpII\n0fxQ8ANAe4hZ7Q7drNJ3gjTcBpUC2JD5Leo31Rpg0Gcg19hCC0Wvgmje3WYkN5Ap\nlBlGGSW4gNfL1IYoakRwJiNiqZ+Gb7+6kHDSVneFeO/qJakXzlByjAA6quPbYzSf\n+AZxAeKCINT+b72x\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIFdDCCBFygAwIBAgIQJ2buVutJ846r13Ci/ITeIjANBgkqhkiG9w0BAQwFADBv\nMQswCQYDVQQGEwJTRTEUMBIGA1UEChMLQWRkVHJ1c3QgQUIxJjAkBgNVBAsTHUFk\nZFRydXN0IEV4dGVybmFsIFRUUCBOZXR3b3JrMSIwIAYDVQQDExlBZGRUcnVzdCBF\neHRlcm5hbCBDQSBSb290MB4XDTAwMDUzMDEwNDgzOFoXDTIwMDUzMDEwNDgzOFow\ngYUxCzAJBgNVBAYTAkdCMRswGQYDVQQIExJHcmVhdGVyIE1hbmNoZXN0ZXIxEDAO\nBgNVBAcTB1NhbGZvcmQxGjAYBgNVBAoTEUNPTU9ETyBDQSBMaW1pdGVkMSswKQYD\nVQQDEyJDT01PRE8gUlNBIENlcnRpZmljYXRpb24gQXV0aG9yaXR5MIICIjANBgkq\nhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAkehUktIKVrGsDSTdxc9EZ3SZKzejfSNw\nAHG8U9/E+ioSj0t/EFa9n3Byt2F/yUsPF6c947AEYe7/EZfH9IY+Cvo+XPmT5jR6\n2RRr55yzhaCCenavcZDX7P0N+pxs+t+wgvQUfvm+xKYvT3+Zf7X8Z0NyvQwA1onr\nayzT7Y+YHBSrfuXjbvzYqOSSJNpDa2K4Vf3qwbxstovzDo2a5JtsaZn4eEgwRdWt\n4Q08RWD8MpZRJ7xnw8outmvqRsfHIKCxH2XeSAi6pE6p8oNGN4Tr6MyBSENnTnIq\nm1y9TBsoilwie7SrmNnu4FGDwwlGTm0+mfqVF9p8M1dBPI1R7Qu2XK8sYxrfV8g/\nvOldxJuvRZnio1oktLqpVj3Pb6r/SVi+8Kj/9Lit6Tf7urj0Czr56ENCHonYhMsT\n8dm74YlguIwoVqwUHZwK53Hrzw7dPamWoUi9PPevtQ0iTMARgexWO/bTouJbt7IE\nIlKVgJNp6I5MZfGRAy1wdALqi2cVKWlSArvX31BqVUa/oKMoYX9w0MOiqiwhqkfO\nKJwGRXa/ghgntNWutMtQ5mv0TIZxMOmm3xaG4Nj/QN370EKIf6MzOi5cHkERgWPO\nGHFrK+ymircxXDpqR+DDeVnWIBqv8mqYqnK8V0rSS527EPywTEHl7R09XiidnMy/\ns1Hap0flhFMCAwEAAaOB9DCB8TAfBgNVHSMEGDAWgBStvZh6NLQm9/rEJlTvA73g\nJMtUGjAdBgNVHQ4EFgQUu69+Aj36pvE8hI6t7jiY7NkyMtQwDgYDVR0PAQH/BAQD\nAgGGMA8GA1UdEwEB/wQFMAMBAf8wEQYDVR0gBAowCDAGBgRVHSAAMEQGA1UdHwQ9\nMDswOaA3oDWGM2h0dHA6Ly9jcmwudXNlcnRydXN0LmNvbS9BZGRUcnVzdEV4dGVy\nbmFsQ0FSb290LmNybDA1BggrBgEFBQcBAQQpMCcwJQYIKwYBBQUHMAGGGWh0dHA6\nLy9vY3NwLnVzZXJ0cnVzdC5jb20wDQYJKoZIhvcNAQEMBQADggEBAGS/g/FfmoXQ\nzbihKVcN6Fr30ek+8nYEbvFScLsePP9NDXRqzIGCJdPDoCpdTPW6i6FtxFQJdcfj\nJw5dhHk3QBN39bSsHNA7qxcS1u80GH4r6XnTq1dFDK8o+tDb5VCViLvfhVdpfZLY\nUspzgb8c8+a4bmYRBbMelC1/kZWSWfFMzqORcUx8Rww7Cxn2obFshj5cqsQugsv5\nB5a6SE2Q8pTIqXOi6wZ7I53eovNNVZ96YUWYGGjHXkBrI/V5eu+MtWuLt29G9Hvx\nPUsE2JOAWVrgQSQdso8VYFhH2+9uRv0V9dlfmrPb2LjkQLPNlzmuhbsdjrzch5vR\npu/xO28QOG8=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIENjCCAx6gAwIBAgIBATANBgkqhkiG9w0BAQUFADBvMQswCQYDVQQGEwJTRTEU\nMBIGA1UEChMLQWRkVHJ1c3QgQUIxJjAkBgNVBAsTHUFkZFRydXN0IEV4dGVybmFs\nIFRUUCBOZXR3b3JrMSIwIAYDVQQDExlBZGRUcnVzdCBFeHRlcm5hbCBDQSBSb290\nMB4XDTAwMDUzMDEwNDgzOFoXDTIwMDUzMDEwNDgzOFowbzELMAkGA1UEBhMCU0Ux\nFDASBgNVBAoTC0FkZFRydXN0IEFCMSYwJAYDVQQLEx1BZGRUcnVzdCBFeHRlcm5h\nbCBUVFAgTmV0d29yazEiMCAGA1UEAxMZQWRkVHJ1c3QgRXh0ZXJuYWwgQ0EgUm9v\ndDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALf3GjPm8gAELTngTlvt\nH7xsD821+iO2zt6bETOXpClMfZOfvUq8k+0DGuOPz+VtUFrWlymUWoCwSXrbLpX9\nuMq/NzgtHj6RQa1wVsfwTz/oMp50ysiQVOnGXw94nZpAPA6sYapeFI+eh6FqUNzX\nmk6vBbOmcZSccbNQYArHE504B4YCqOmoaSYYkKtMsE8jqzpPhNjfzp/haW+710LX\na0Tkx63ubUFfclpxCDezeWWkWaCUN/cALw3CknLa0Dhy2xSoRcRdKn23tNbE7qzN\nE0S3ySvdQwAl+mG5aWpYIxG3pzOPVnVZ9c0p10a3CitlttNCbxWyuHv77+ldU9U0\nWicCAwEAAaOB3DCB2TAdBgNVHQ4EFgQUrb2YejS0Jvf6xCZU7wO94CTLVBowCwYD\nVR0PBAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wgZkGA1UdIwSBkTCBjoAUrb2YejS0\nJvf6xCZU7wO94CTLVBqhc6RxMG8xCzAJBgNVBAYTAlNFMRQwEgYDVQQKEwtBZGRU\ncnVzdCBBQjEmMCQGA1UECxMdQWRkVHJ1c3QgRXh0ZXJuYWwgVFRQIE5ldHdvcmsx\nIjAgBgNVBAMTGUFkZFRydXN0IEV4dGVybmFsIENBIFJvb3SCAQEwDQYJKoZIhvcN\nAQEFBQADggEBALCb4IUlwtYj4g+WBpKdQZic2YR5gdkeWxQHIzZlj7DYd7usQWxH\nYINRsPkyPef89iYTx4AWpb9a/IfPeHmJIZriTAcKhjW88t5RxNKWt9x+Tu5w/Rw5\n6wwCURQtjr0W4MHfRnXnJK3s9EK0hZNwEGe6nQY1ShjTK3rMUUKhemPR5ruhxSvC\nNr4TDea9Y355e6cJDUCrat2PisP29owaQgVR1EX1n6diIWgVIEM8med8vSTYqZEX\nc4g/VhsxOBi0cQ+azcgOno4uG+GMmIPLHzHxREzGBHNJdmAPx/i9F4BrLunMTA5a\nmnkPIAou1Z5jJh5VkpTYghdae9C8x49OhgQ=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIKuzCCCaOgAwIBAgIMNjYXNLLXqLY/TOgcMA0GCSqGSIb3DQEBCwUAMFcxCzAJ\nBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9iYWxTaWduIG52LXNhMS0wKwYDVQQDEyRH\nbG9iYWxTaWduIENsb3VkU1NMIENBIC0gU0hBMjU2IC0gRzMwHhcNMTcwMzIzMTEy\nMzE5WhcNMTcxMTAzMTEyMjIzWjBgMQswCQYDVQQGEwJVUzERMA8GA1UECBMIRGVs\nYXdhcmUxDjAMBgNVBAcTBURvdmVyMRYwFAYDVQQKEw1JbmNhcHN1bGEgSW5jMRYw\nFAYDVQQDEw1pbmNhcHN1bGEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB\nCgKCAQEA3sgLKGR+l4KI2c5rjhwTQcuXu32CXy2BuUqZkco4pDsHzvBWKNAzxqSe\nUOt4LmhBzLETdHqvsXVPnNW4mbruVbreTZoJ6//Jy4WfBAtvVq/uL9krmv19opt1\n0Ll4+LLpeI+VBnkCNBPrALk/7aoVHeZcTmRmq2QPbRIBSaa+ZQhL+swQc+ATmcxd\nz9ew1gZPo9kWC4lZ63FIRu97wa3yKWi1qD8alv1EKFgInZOLtp359BOJlLvTt4zC\nnAzvMZog8YXJ9Mit5mSWTBGfKp0Z4QF5GIQNcM2rPFZC3nWjwjp7DwgsCvpJDzHR\nkpULclhrBX7LvbvMdMzT2KgvvrwZtwIDAQABo4IHfDCCB3gwDgYDVR0PAQH/BAQD\nAgWgMIGKBggrBgEFBQcBAQR+MHwwQgYIKwYBBQUHMAKGNmh0dHA6Ly9zZWN1cmUu\nZ2xvYmFsc2lnbi5jb20vY2FjZXJ0L2Nsb3Vkc3Nsc2hhMmczLmNydDA2BggrBgEF\nBQcwAYYqaHR0cDovL29jc3AyLmdsb2JhbHNpZ24uY29tL2Nsb3Vkc3Nsc2hhMmcz\nMFYGA1UdIARPME0wQQYJKwYBBAGgMgEUMDQwMgYIKwYBBQUHAgEWJmh0dHBzOi8v\nd3d3Lmdsb2JhbHNpZ24uY29tL3JlcG9zaXRvcnkvMAgGBmeBDAECAjAJBgNVHRME\nAjAAMIIGFQYDVR0RBIIGDDCCBgiCDWluY2Fwc3VsYS5jb22CHSouYWNjZXB0YXRp\nZS1lbmdpZS1lbmVyZ2llLm5sgg0qLmFtd2FsYWsuY29tghsqLmFwcGx5LmdvbWFz\ndGVyY2FyZC5jb20uYXWCCyouYXZpdmEuY29tghEqLmF2aXZhY2FuYWRhLmNvbYIQ\nKi5iaW5nb21hbmlhLmNvbYIRKi5icmFuY2hldHZvdXMuZnKCEyouY2xlYXJza3kt\nZGF0YS5uZXSCDiouY29uZWN0eXMuY29tghAqLmNvbnRlbnRmdWwuY29tgg4qLmNv\ncmVmb3VyLmNvbYIMKi5jb3JzYWlyLmNpggwqLmNvcnNhaXIuZ3CCDCouY29yc2Fp\nci5tcYIMKi5jb3JzYWlyLnNughQqLmNyZWRpdG95Y2F1Y2lvbi5lc4IWKi5kYzQu\ncGFnZXVwcGVvcGxlLmNvbYIWKi5kZXZpY2Vwcm90ZWN0aW9uLmNvbYISKi5kaWFq\ndWdvc28xMjMuY29tghUqLmRpcmVjdG1vYmlsZXMuY28udWuCEyouZWRkaWVhbmRj\nby5jb20uYXWCFCouZWtlZXBlcmdyb3VwLmNvLnVrggwqLmVwaWR1by5jb22CESou\nZXBpZHVvZm9ydGUuY29tghUqLmV2b3F1YWFkdmFudGFnZS5jb22CFSouaGVkZ2Vz\ndG9uZWdyb3VwLmNvbYIUKi5pbmNhcHN1bGEtZGVtby5iaXqCDCouaXZyYXBwLm5l\ndIINKi5rcC1teXBnLmNvbYIYKi5sYi5uZXN0bGUtd2F0ZXJzbmEuY29tgg0qLmx1\nbHVjcm0uY29tghMqLm1hZGV3aXRobmVzdGxlLmNhgg4qLm1hcmljb3BhLmVkdYIU\nKi5tYXR0ZWxwYXJ0bmVycy5jb22CGioubXhwLnphbXNoLmluY2Fwc3VsYS5tb2Jp\ngg0qLm15ZXIuY29tLmF1gg0qLm15bnZhcHAuY29tgg8qLm5ldDJwaG9uZS5jb22C\nDSoucGluMTExMS5jb22CECoucHJvZC5pbG9hbi5jb22CGCouc2Nhbm5lci5zcG90\nb3B0aW9uLmNvbYIZKi5zZWFyY2hmbG93c3RhZ2luZy5jby51a4IXKi5zaW1wbHli\nZXR0ZXJ0aW4uY28udWuCCiouc29mbi5jb22CDiouc3RyYXR0b24uY29tghcqLnRl\nc3QtZW5naWUtZW5lcmdpZS5ubIIQKi50cmF2ZWxwb3J0LmNvbYIOKi50cmVtYmxh\nbnQuY2GCCyoudml0dGVsLmpwgg0qLnZ0ZWNoLmNvLnVrgg0qLndlcmFsbHkuY29t\nghcqLndoaXRlaG91c2VoaXN0b3J5Lm9yZ4IMKi53cnBzLm9uLmNhggkqLnd0ZS5u\nZXSCCyoueW91ZmkuY29tghthY2NlcHRhdGllLWVuZ2llLWVuZXJnaWUubmyCC2Ft\nd2FsYWsuY29tgglhdml2YS5jb22CDmJpbmdvbWFuaWEuY29tgg9icmFuY2hldHZv\ndXMuZnKCDGNvbmVjdHlzLmNvbYIKY29yc2Fpci5jaYIKY29yc2Fpci5ncIIKY29y\nc2Fpci5tcYIKY29yc2Fpci5zboIUZGV2aWNlcHJvdGVjdGlvbi5jb22CEGRpYWp1\nZ29zbzEyMy5jb22CE2RpcmVjdG1vYmlsZXMuY28udWuCEWVkZGllYW5kY28uY29t\nLmF1ggplcGlkdW8uY29tgg9lcGlkdW9mb3J0ZS5jb22CE2V2b3F1YWFkdmFudGFn\nZS5jb22CE2hlZGdlc3RvbmVncm91cC5jb22CC2twLW15cGcuY29tghFtYWRld2l0\naG5lc3RsZS5jYYINbmV0MnBob25lLmNvbYILcGluMTExMS5jb22CF3NlYXJjaGZs\nb3dzdGFnaW5nLmNvLnVrghVzaW1wbHliZXR0ZXJ0aW4uY28udWuCFXRlc3QtZW5n\naWUtZW5lcmdpZS5ubIIOdHJhdmVscG9ydC5jb22CC3Z0ZWNoLmNvLnVrggt3ZXJh\nbGx5LmNvbYIKd3Jwcy5vbi5jYYIHd3RlLm5ldDAdBgNVHSUEFjAUBggrBgEFBQcD\nAQYIKwYBBQUHAwIwHQYDVR0OBBYEFFAxG/f60PL3CFmmnW/Uk9gBtDhCMB8GA1Ud\nIwQYMBaAFKkrh+HOJEc7G7/PhTcCVZ0NlFjmMA0GCSqGSIb3DQEBCwUAA4IBAQAn\n/+V5a6hfMgXZJa5H0c0Gu5E3bSl8gvQsS4VsT1tnI0OnjwqQtd4fRC2TQFogSYbk\nDfmtFxiiQymF9CtlRbTQX41gZ6RMLRIyeA96k7WC3PJiIlqiFDp0172INTU0NsiX\nfJ/u2plLINtye67yUt38TGOYZa1aF/mjAtN+tubumY1Va7k/ec4b+qhZQUOzgGZv\nTrx5wDYy8UBeUkqn7/ZVH7FDAN6wcc97e49/02okiH7pu9bSlP9Izl6YSaLBRcAy\nTAixepqGxVjhK1OmKMIGrIiI2H9oEelpNImMvgQo8XK+2Bcvnw/H5qQg8VmFwPd7\ndfeszhlA0vV4mgGQMm5T\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIEizCCA3OgAwIBAgIORvCM288sVGbvMwHdXzQwDQYJKoZIhvcNAQELBQAwVzEL\nMAkGA1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsT\nB1Jvb3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw0xNTA4MTkw\nMDAwMDBaFw0yNTA4MTkwMDAwMDBaMFcxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBH\nbG9iYWxTaWduIG52LXNhMS0wKwYDVQQDEyRHbG9iYWxTaWduIENsb3VkU1NMIENB\nIC0gU0hBMjU2IC0gRzMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCj\nwHXhMpjl2a6EfI3oI19GlVtMoiVw15AEhYDJtfSKZU2Sy6XEQqC2eSUx7fGFIM0T\nUT1nrJdNaJszhlyzey2q33egYdH1PPua/NPVlMrJHoAbkJDIrI32YBecMbjFYaLi\nblclCG8kmZnPlL/Hi2uwH8oU+hibbBB8mSvaSmPlsk7C/T4QC0j0dwsv8JZLOu69\nNd6FjdoTDs4BxHHT03fFCKZgOSWnJ2lcg9FvdnjuxURbRb0pO+LGCQ+ivivc41za\nWm+O58kHa36hwFOVgongeFxyqGy+Z2ur5zPZh/L4XCf09io7h+/awkfav6zrJ2R7\nTFPrNOEvmyBNVBJrfSi9AgMBAAGjggFTMIIBTzAOBgNVHQ8BAf8EBAMCAQYwHQYD\nVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMBIGA1UdEwEB/wQIMAYBAf8CAQAw\nHQYDVR0OBBYEFKkrh+HOJEc7G7/PhTcCVZ0NlFjmMB8GA1UdIwQYMBaAFGB7ZhpF\nDZfKiVAvfQTNNKj//P1LMD0GCCsGAQUFBwEBBDEwLzAtBggrBgEFBQcwAYYhaHR0\ncDovL29jc3AuZ2xvYmFsc2lnbi5jb20vcm9vdHIxMDMGA1UdHwQsMCowKKAmoCSG\nImh0dHA6Ly9jcmwuZ2xvYmFsc2lnbi5jb20vcm9vdC5jcmwwVgYDVR0gBE8wTTAL\nBgkrBgEEAaAyARQwPgYGZ4EMAQICMDQwMgYIKwYBBQUHAgEWJmh0dHBzOi8vd3d3\nLmdsb2JhbHNpZ24uY29tL3JlcG9zaXRvcnkvMA0GCSqGSIb3DQEBCwUAA4IBAQCi\nHWmKCo7EFIMqKhJNOSeQTvCNrNKWYkc2XpLR+sWTtTcHZSnS9FNQa8n0/jT13bgd\n+vzcFKxWlCecQqoETbftWNmZ0knmIC/Tp3e4Koka76fPhi3WU+kLk5xOq9lF7qSE\nhf805A7Au6XOX5WJhXCqwV3szyvT2YPfA8qBpwIyt3dhECVO2XTz2XmCtSZwtFK8\njzPXiq4Z0PySrS+6PKBIWEde/SBWlSDBch2rZpmk1Xg3SBufskw3Z3r9QtLTVp7T\nHY7EDGiWtkdREPd76xUJZPX58GMWLT3fI0I6k2PMq69PVwbH/hRVYs4nERnh9ELt\nIjBrNRpKBYCkZd/My2/Q\n-----END CERTIFICATE-----\n"))
	return &ManagementClient{
		authToken: authToken,
		client:    &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}},
		host:      fmt.Sprintf("https://%s", contentfulCPAURL),
		pool:      pool,
		spaceID:   "ygx37epqlss8",
	}
}

// Webhook describes a webhook definition
type Webhook struct {
	ID      string   `json:"-"`
	Version int      `json:"-"`
	URL     string   `json:"url"`
	Name    string   `json:"name"`
	Topics  []string `json:"topics"`
}

// WebhookIterator is used to paginate webhooks
type WebhookIterator struct {
	Page   int
	Limit  int
	Offset int
	c      *ManagementClient
	items  []Webhook
}
type webhookItem struct {
	Sys sys `json:"sys"`
	Webhook
}
type webhooksResponse struct {
	Total int           `json:"total"`
	Skip  int           `json:"skip"`
	Limit int           `json:"limit"`
	Items []webhookItem `json:"items"`
}

// Next returns the following item of type Webhook. If none exists a network request will be executed
func (it *WebhookIterator) Next() (*Webhook, error) {
	if len(it.items) == 0 {
		if err := it.fetch(); err != nil {
			return nil, err
		}
	}
	if len(it.items) == 0 {
		return nil, ErrIteratorDone
	}
	var item Webhook
	item, it.items = it.items[len(it.items)-1], it.items[:len(it.items)-1]
	if len(it.items) == 0 {
		it.Page++
		it.Offset = it.Page * it.Limit
	}
	return &item, nil
}
func (it *WebhookIterator) fetch() error {
	c := it.c
	var url = fmt.Sprintf("https://api.contentful.com/spaces/%s/webhook_definitions?limit=%d&skip=%d", c.spaceID, it.Limit, it.Offset)
	var req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	req.Header.Set("Content-Type", "application/vnd.contentful.management.v1+json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	var data webhooksResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	it.items = []Webhook{}
	for _, i := range data.Items {
		i.Webhook.ID = i.Sys.ID
		i.Webhook.Version = i.Sys.Version
		it.items = append(it.items, i.Webhook)
	}
	return nil
}

// List retrieves paginated webhooks
func (ws *WebhookService) List(opts ListOptions) *WebhookIterator {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	it := &WebhookIterator{
		Limit: opts.Limit,
		Page:  opts.Page,
		c:     ws.client,
	}
	return it
}

// Create adds a new webhook definitions
func (ws *WebhookService) Create(w *Webhook) error {
	var url = fmt.Sprintf("https://api.contentful.com/spaces/%s/webhook_definitions", ws.client.spaceID)
	b := bytes.Buffer{}
	var payload = webhookItem{Webhook: *w}
	if err := json.NewEncoder(&b).Encode(payload); err != nil {
		return err
	}
	var req, err = http.NewRequest("POST", url, &b)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ws.client.authToken))
	req.Header.Set("Content-Type", "application/vnd.contentful.management.v1+json")
	resp, err := ws.client.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	w.ID = payload.Sys.ID
	w.Version = payload.Sys.Version
	return nil
}

// Update changes an existing webhook definitions
func (ws *WebhookService) Update(w *Webhook) error {
	var url = fmt.Sprintf("https://api.contentful.com/spaces/%s/webhook_definitions/%s", ws.client.spaceID, w.ID)
	b := bytes.Buffer{}
	var payload = webhookItem{Webhook: *w}
	if err := json.NewEncoder(&b).Encode(payload); err != nil {
		return err
	}
	var req, err = http.NewRequest("PUT", url, &b)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ws.client.authToken))
	req.Header.Set("X-Contentful-Version", fmt.Sprintf("%d", w.Version))
	req.Header.Set("Content-Type", "application/vnd.contentful.management.v1+json")
	resp, err := ws.client.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	*w = payload.Webhook
	return nil
}

// Delete adds a new webhook definitions
func (ws *WebhookService) Delete(id string) error {
	var url = fmt.Sprintf("https://api.contentful.com/spaces/%s/webhook_definitions/%s", ws.client.spaceID, id)
	var req, err = http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ws.client.authToken))
	req.Header.Set("Content-Type", "application/vnd.contentful.management.v1+json")
	resp, err := ws.client.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Request failed: %s, %v", resp.Status, err)
	}
	return resp.Body.Close()
}

// WebhookService includes webhook management functions
type WebhookService struct {
	client *ManagementClient
}

// Webhooks returns a Webhook management service
func (c *ManagementClient) Webhooks() *WebhookService {
	return &WebhookService{client: c}
}

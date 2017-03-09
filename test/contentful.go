package main

import (
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
	c            *Client
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
		return nil, IteratorDone
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
	var url = fmt.Sprintf("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=%d&locale=%s&limit=%d&skip=%d", c.host, c.spaceID, c.authToken, "2wKn6yEnZewu2SCCkus4as", it.IncludeCount, c.Locales[0], it.Limit, it.Offset)
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
		json.Unmarshal(*raw.Fields, &item.Fields)
		items[i] = &Post{
			Approver:      resolveAuthor(item.Fields.Approver.Sys.ID, data.Items, data.Includes, it.lookupCache),
			Author:        resolveAuthors(item.Fields.Author, data.Items, data.Includes, it.lookupCache),
			AuthorOrPost:  resolveEntries(item.Fields.AuthorOrPost, data.Items, data.Includes, it.lookupCache),
			Body:          item.Fields.Body,
			Category:      resolveCategorys(item.Fields.Category, data.Items, data.Includes, it.lookupCache),
			Comments:      item.Fields.Comments,
			Date:          item.Fields.Date,
			FeaturedImage: resolveAsset(item.Fields.FeaturedImage.Sys.ID, data.Includes),
			ID:            item.Sys.ID,
			Slug:          item.Fields.Slug,
			Title:         item.Fields.Title,
		}
	}
	it.items = items
	return nil
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
				ID:       item.Sys.ID,
				Slug:     item.Fields.Slug,
				Title:    item.Fields.Title,
			}
			cache.posts[entry.Sys.ID] = tmp
			tmp.Author = resolveAuthors(item.Fields.Author, items, includes, cache)
			tmp.Category = resolveCategorys(item.Fields.Category, items, includes, cache)
			tmp.FeaturedImage = resolveAsset(item.Fields.FeaturedImage.Sys.ID, includes)
			tmp.Approver = resolveAuthor(item.Fields.Approver.Sys.ID, items, includes, cache)
			tmp.AuthorOrPost = resolveEntries(item.Fields.AuthorOrPost, items, includes, cache)
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
	for _, entry := range items {
		ptrs = append(ptrs, &entry)
	}
	return ptrs
}
func resolvePosts(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []Post {
	var items []Post
	for _, entry := range append(includes.Entries, its...) {
		var item postItem
		var included = false
		for _, entryID := range ids {
			included = included || entryID.Sys.ID == entry.Sys.ID
		}
		if included == true {
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
				ID:       item.Sys.ID,
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
	}
	return items
}

// Posts retrieves paginated Post entries
func (c *Client) Posts(opts ListOptions) *PostIterator {
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
	c            *Client
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
		return nil, IteratorDone
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
	var url = fmt.Sprintf("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=%d&locale=%s&limit=%d&skip=%d", c.host, c.spaceID, c.authToken, "1kUEViTN4EmGiEaaeC6ouY", it.IncludeCount, c.Locales[0], it.Limit, it.Offset)
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
		json.Unmarshal(*raw.Fields, &item.Fields)
		items[i] = &Author{
			Age:            item.Fields.Age,
			Biography:      item.Fields.Biography,
			CreatedEntries: resolvePosts(item.Fields.CreatedEntries, data.Items, data.Includes, it.lookupCache),
			ID:             item.Sys.ID,
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
				ID:        item.Sys.ID,
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
	for _, entry := range items {
		ptrs = append(ptrs, &entry)
	}
	return ptrs
}
func resolveAuthors(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []Author {
	var items []Author
	for _, entry := range append(includes.Entries, its...) {
		var item authorItem
		var included = false
		for _, entryID := range ids {
			included = included || entryID.Sys.ID == entry.Sys.ID
		}
		if included == true {
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
				ID:        item.Sys.ID,
				Name:      item.Fields.Name,
				Rating:    item.Fields.Rating,
				Website:   item.Fields.Website,
			}
			cache.authors[entry.Sys.ID] = tmp
			tmp.ProfilePhoto = resolveAsset(item.Fields.ProfilePhoto.Sys.ID, includes)
			tmp.CreatedEntries = resolvePosts(item.Fields.CreatedEntries, its, includes, cache)
			items = append(items, *tmp)
		}
	}
	return items
}

// Authors retrieves paginated Author entries
func (c *Client) Authors(opts ListOptions) *AuthorIterator {
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
	c            *Client
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
		return nil, IteratorDone
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
	var url = fmt.Sprintf("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=%d&locale=%s&limit=%d&skip=%d", c.host, c.spaceID, c.authToken, "5KMiN6YPvi42icqAUQMCQe", it.IncludeCount, c.Locales[0], it.Limit, it.Offset)
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
		json.Unmarshal(*raw.Fields, &item.Fields)
		items[i] = &Category{
			ID:               item.Sys.ID,
			Icon:             resolveAsset(item.Fields.Icon.Sys.ID, data.Includes),
			Parent:           resolveCategoryPtr(item.Fields.Parent.Sys.ID, data.Items, data.Includes, it.lookupCache),
			ShortDescription: item.Fields.ShortDescription,
			Title:            item.Fields.Title,
		}
	}
	it.items = items
	return nil
}

// Category
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
				ID:               item.Sys.ID,
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
	for _, entry := range items {
		ptrs = append(ptrs, &entry)
	}
	return ptrs
}
func resolveCategorys(ids entryIDs, its []includeEntry, includes includes, cache *iteratorCache) []Category {
	var items []Category
	for _, entry := range append(includes.Entries, its...) {
		var item categoryItem
		var included = false
		for _, entryID := range ids {
			included = included || entryID.Sys.ID == entry.Sys.ID
		}
		if included == true {
			if v, ok := cache.categorys[entry.Sys.ID]; ok {
				items = append(items, *v)
				continue
			}
			if err := json.Unmarshal(*entry.Fields, &item.Fields); err != nil {
				return items
			}
			var tmp = &Category{
				ID:               item.Sys.ID,
				ShortDescription: item.Fields.ShortDescription,
				Title:            item.Fields.Title,
			}
			cache.categorys[entry.Sys.ID] = tmp
			tmp.Parent = resolveCategoryPtr(item.Fields.Parent.Sys.ID, its, includes, cache)
			tmp.Icon = resolveAsset(item.Fields.Icon.Sys.ID, includes)
			items = append(items, *tmp)
		}
	}
	return items
}

// Categories retrieves paginated Category entries
func (c *Client) Categories(opts ListOptions) *CategoryIterator {
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

var IteratorDone error = fmt.Errorf("IteratorDone")

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

// Client
type Client struct {
	host      string
	spaceID   string
	authToken string
	Locales   []string
	client    *http.Client
	pool      *x509.CertPool
}

const ContentfulCDNURL = "cdn.contentful.com"

// New returns a contentful client
func New(authToken string, locales []string) *Client {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte("-----BEGIN CERTIFICATE-----\nMIIL6TCCCtGgAwIBAgIQBigdNnW0H8yz/xj67Pj93zANBgkqhkiG9w0BAQsFADBw\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3\nd3cuZGlnaWNlcnQuY29tMS8wLQYDVQQDEyZEaWdpQ2VydCBTSEEyIEhpZ2ggQXNz\ndXJhbmNlIFNlcnZlciBDQTAeFw0xNDEyMDgwMDAwMDBaFw0xODAyMDYxMjAwMDBa\nMGwxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMRUwEwYDVQQKEwxGYXN0bHksIEluYy4xGTAXBgNVBAMTEGEu\nc3NsLmZhc3RseS5uZXQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDU\nJUiQsaVP/vC4Mb3aJUmA9KnMQa7EJfjYLsE4F0VehrOp8jlSSXmQLELlUAwPp2F2\nPNyB32DDOFBHZIYwFrApFEzsJdTKQUYk6xHPZOdYoIijpmfb5xRMdTjqxThGkk+k\nhU0+ipPWiErJNRkapLgPwPD4ctd5X8rnKF8lMHIxx5Xhdg6PqZC3F7y45Nym2a3M\n8xIKIkB77o1bkuDpGnV9ZESC/Yf9Mc4NmWrQjqQc+8yIabir+n7/YcM5UdUjZPNS\nhgL4jLYVJ+KDRZcjIT/dXRZoPpJgRFL9NIep/eSAzQa3g659uW7tjN6tg5iQm4hw\nksaWp+zfTAJc4IXNtlndAgMBAAGjggiBMIIIfTAfBgNVHSMEGDAWgBRRaP+QrwIH\ndTzM2WVkYqISuFlyOzAdBgNVHQ4EFgQUwIj0Y03ka1Q28RLCtKWy4nN7FIgwggax\nBgNVHREEggaoMIIGpIIQYS5zc2wuZmFzdGx5Lm5ldIISKi5hLnNzbC5mYXN0bHku\nbmV0gg9mYXN0Lndpc3RpYS5jb22CEHB1cmdlLmZhc3RseS5uZXSCEm1pcnJvcnMu\nZmFzdGx5Lm5ldIIOKi5wYXJzZWNkbi5jb22CDSouZmFzdHNzbC5uZXSCCXZveGVy\nLmNvbYINd3d3LnZveGVyLmNvbYIOKi5maXJlYmFzZS5jb22CEHNpdGVzLnlhbW1l\nci5jb22CGHNpdGVzLnN0YWdpbmcueWFtbWVyLmNvbYIPKi5za2ltbGlua3MuY29t\nghMqLnNraW1yZXNvdXJjZXMuY29tghBjZG4udGhpbmdsaW5rLm1lggwqLmZpdGJp\ndC5jb22CEiouaG9zdHMuZmFzdGx5Lm5ldIISY29udHJvbC5mYXN0bHkubmV0gg8q\nLndpa2lhLWluYy5jb22CFSoucGVyZmVjdGF1ZGllbmNlLmNvbYILKi53aWtpYS5j\nb22CEmYuY2xvdWQuZ2l0aHViLmNvbYIVKi5kaWdpdGFsc2Npcm9jY28ubmV0ggoq\nLmV0c3kuY29tghAqLmV0c3lzdGF0aWMuY29tgg0qLmFkZHRoaXMuY29tghAqLmFk\nZHRoaXNjZG4uY29tgg9mYXN0Lndpc3RpYS5uZXSCDnJhdy5naXRodWIuY29tgg93\nd3cudXNlcmZveC5jb22CEyouYXNzZXRzLXlhbW1lci5jb22CGyouc3RhZ2luZy5h\nc3NldHMteWFtbWVyLmNvbYIWYXNzZXRzLmh1Z2dpZXMtY2RuLm5ldIISb3JiaXQu\nc2hhemFtaWQuY29tgg9hYm91dC5qc3Rvci5vcmeCFyouZ2xvYmFsLnNzbC5mYXN0\nbHkubmV0gg13ZWIudm94ZXIuY29tgg9weXBpLnB5dGhvbi5vcmeCCyouMTJ3YnQu\nY29tghJ3d3cuaG9sZGVyZGVvcmQubm+CGnNlY3VyZWQuaW5kbi5pbmZvbGlua3Mu\nY29tghBwbGF5LnZpZHlhcmQuY29tghhwbGF5LXN0YWdpbmcudmlkeWFyZC5jb22C\nFXNlY3VyZS5pbWcud2ZyY2RuLmNvbYIWc2VjdXJlLmltZy5qb3NzY2RuLmNvbYIQ\nKi5nb2NhcmRsZXNzLmNvbYIVd2lkZ2V0cy5waW50ZXJlc3QuY29tgg4qLjdkaWdp\ndGFsLmNvbYINKi43c3RhdGljLmNvbYIPcC5kYXRhZG9naHEuY29tghBuZXcubXVs\nYmVycnkuY29tghJ3d3cuc2FmYXJpZmxvdy5jb22CEmNkbi5jb250ZW50ZnVsLmNv\nbYIQdG9vbHMuZmFzdGx5Lm5ldIISKi5odWV2b3NidWVub3MuY29tgg4qLmdvb2Rl\nZ2dzLmNvbYIWKi5mYXN0bHkucGljbW9ua2V5LmNvbYIVKi5jZG4ud2hpcHBsZWhp\nbGwubmV0ghEqLndoaXBwbGVoaWxsLm5ldIIbY2RuLm1lZGlhMzQud2hpcHBsZWhp\nbGwubmV0ghtjZG4ubWVkaWE1Ni53aGlwcGxlaGlsbC5uZXSCG2Nkbi5tZWRpYTc4\nLndoaXBwbGVoaWxsLm5ldIIcY2RuLm1lZGlhOTEwLndoaXBwbGVoaWxsLm5ldIIO\nKi5tb2RjbG90aC5jb22CDyouZGlzcXVzY2RuLmNvbYILKi5qc3Rvci5vcmeCDyou\nZHJlYW1ob3N0LmNvbYIOd3d3LmZsaW50by5jb22CDyouY2hhcnRiZWF0LmNvbYIN\nKi5oaXBtdW5rLmNvbYIaY29udGVudC5iZWF2ZXJicm9va3MuY28udWuCG3NlY3Vy\nZS5jb21tb24uY3Nuc3RvcmVzLmNvbYIOd3d3LmpvaW5vcy5jb22CJXN0YWdpbmct\nbW9iaWxlLWNvbGxlY3Rvci5uZXdyZWxpYy5jb22CDioubW9kY2xvdGgubmV0ghAq\nLmZvdXJzcXVhcmUuY29tggwqLnNoYXphbS5jb22CCiouNHNxaS5uZXSCDioubWV0\nYWNwYW4ub3JnggwqLmZhc3RseS5jb22CCXdpa2lhLmNvbYIKZmFzdGx5LmNvbYIR\nKi5nYWR2ZW50dXJlcy5jb22CFnd3dy5nYWR2ZW50dXJlcy5jb20uYXWCFXd3dy5n\nYWR2ZW50dXJlcy5jby51a4IJa3JlZG8uY29tghZjZG4tdGFncy5icmFpbmllbnQu\nY29tghRteS5iaWxsc3ByaW5nYXBwLmNvbYIGcnZtLmlvMA4GA1UdDwEB/wQEAwIF\noDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwdQYDVR0fBG4wbDA0oDKg\nMIYuaHR0cDovL2NybDMuZGlnaWNlcnQuY29tL3NoYTItaGEtc2VydmVyLWc1LmNy\nbDA0oDKgMIYuaHR0cDovL2NybDQuZGlnaWNlcnQuY29tL3NoYTItaGEtc2VydmVy\nLWc1LmNybDBMBgNVHSAERTBDMDcGCWCGSAGG/WwBATAqMCgGCCsGAQUFBwIBFhxo\ndHRwczovL3d3dy5kaWdpY2VydC5jb20vQ1BTMAgGBmeBDAECAjCBgwYIKwYBBQUH\nAQEEdzB1MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2VydC5jb20wTQYI\nKwYBBQUHMAKGQWh0dHA6Ly9jYWNlcnRzLmRpZ2ljZXJ0LmNvbS9EaWdpQ2VydFNI\nQTJIaWdoQXNzdXJhbmNlU2VydmVyQ0EuY3J0MAwGA1UdEwEB/wQCMAAwDQYJKoZI\nhvcNAQELBQADggEBAKLWzbX7wSyjzE7BVMjLrHAaiz+WGSwrAPrQBJ29sqouu9gv\nI7i2Ie6eiRb4YLMouy6D+ZNZ+RM+Hkjv+PZFxCcDRmaWi+74ha5d8O155gRJRPZ0\nSy5SfD/8kqrJRfC+/D/KdQzOroD4sx6Qprs9lZ0IEn4CTf0YPNV+Cps37LsVyPJL\nfjDlGIM5K3B/vtZfn2f8buQ9QyKiN0bc67GdCjih9dSrkQNkxJiEOwqiSjYtkdFO\ndYpXF8d1rQKV7a6z2vJloDwilfXLLlUX7rA3qVu7r4EUfIsZgH7hgB4bbst7tx+7\nPgUEq2334kKPVFpsxgsj5++k4lh7tNlakXiBUtw=\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIEsTCCA5mgAwIBAgIQBOHnpNxc8vNtwCtCuF0VnzANBgkqhkiG9w0BAQsFADBs\nMQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3\nd3cuZGlnaWNlcnQuY29tMSswKQYDVQQDEyJEaWdpQ2VydCBIaWdoIEFzc3VyYW5j\nZSBFViBSb290IENBMB4XDTEzMTAyMjEyMDAwMFoXDTI4MTAyMjEyMDAwMFowcDEL\nMAkGA1UEBhMCVVMxFTATBgNVBAoTDERpZ2lDZXJ0IEluYzEZMBcGA1UECxMQd3d3\nLmRpZ2ljZXJ0LmNvbTEvMC0GA1UEAxMmRGlnaUNlcnQgU0hBMiBIaWdoIEFzc3Vy\nYW5jZSBTZXJ2ZXIgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC2\n4C/CJAbIbQRf1+8KZAayfSImZRauQkCbztyfn3YHPsMwVYcZuU+UDlqUH1VWtMIC\nKq/QmO4LQNfE0DtyyBSe75CxEamu0si4QzrZCwvV1ZX1QK/IHe1NnF9Xt4ZQaJn1\nitrSxwUfqJfJ3KSxgoQtxq2lnMcZgqaFD15EWCo3j/018QsIJzJa9buLnqS9UdAn\n4t07QjOjBSjEuyjMmqwrIw14xnvmXnG3Sj4I+4G3FhahnSMSTeXXkgisdaScus0X\nsh5ENWV/UyU50RwKmmMbGZJ0aAo3wsJSSMs5WqK24V3B3aAguCGikyZvFEohQcft\nbZvySC/zA/WiaJJTL17jAgMBAAGjggFJMIIBRTASBgNVHRMBAf8ECDAGAQH/AgEA\nMA4GA1UdDwEB/wQEAwIBhjAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIw\nNAYIKwYBBQUHAQEEKDAmMCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2Vy\ndC5jb20wSwYDVR0fBEQwQjBAoD6gPIY6aHR0cDovL2NybDQuZGlnaWNlcnQuY29t\nL0RpZ2lDZXJ0SGlnaEFzc3VyYW5jZUVWUm9vdENBLmNybDA9BgNVHSAENjA0MDIG\nBFUdIAAwKjAoBggrBgEFBQcCARYcaHR0cHM6Ly93d3cuZGlnaWNlcnQuY29tL0NQ\nUzAdBgNVHQ4EFgQUUWj/kK8CB3U8zNllZGKiErhZcjswHwYDVR0jBBgwFoAUsT7D\naQP4v0cB1JgmGggC72NkK8MwDQYJKoZIhvcNAQELBQADggEBABiKlYkD5m3fXPwd\naOpKj4PWUS+Na0QWnqxj9dJubISZi6qBcYRb7TROsLd5kinMLYBq8I4g4Xmk/gNH\nE+r1hspZcX30BJZr01lYPf7TMSVcGDiEo+afgv2MW5gxTs14nhr9hctJqvIni5ly\n/D6q1UEL2tU2ob8cbkdJf17ZSHwD2f2LSaCYJkJA69aSEaRkCldUxPUd1gJea6zu\nxICaEnL6VpPX/78whQYwvwt/Tv9XBZ0k7YXDK/umdaisLRbvfXknsuvCnQsH6qqF\n0wGjIChBWUMo0oHjqvbsezt3tkBigAVBRQHvFwY+3sAzm2fTYS5yh+Rp/BIAV0Ae\ncPUeybQ=\n-----END CERTIFICATE-----\n"))
	return &Client{
		Locales:   locales,
		authToken: authToken,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: pool,
				},
			},
		},
		host:    fmt.Sprintf("https://%s", ContentfulCDNURL),
		pool:    pool,
		spaceID: "ygx37epqlss8",
	}
}

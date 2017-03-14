package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dave/jennifer/jen"
)

type validation struct {
	LinkContentType []string `json:"linkContentType"`
}

type field struct {
	Name      string `json:"id"`
	Type      string `json:"type"`
	LinkType  string `json:"linkType"`
	Localized bool   `json:"localized"`
	Required  bool   `json:"required"`
	Disabled  bool   `json:"disabled"`
	Items     struct {
		Type        string       `json:"type"`
		LinkType    string       `json:"linkType"`
		Validations []validation `json:"validations"`
	}
	Validations []validation `json:"validations"`
}

type contentfulModel struct {
	Description  string  `json:"description"`
	Name         string  `json:"name"`
	DisplayField string  `json:"displayField"`
	Fields       []field `json:"fields"`
	Sys          struct {
		ID string
	}
}

func (m contentfulModel) CapitalizedName() string {
	return strings.ToUpper(m.Name[0:1]) + m.Name[1:]
}

func (m contentfulModel) DowncasedName() string {
	return strings.ToLower(m.Name[0:1]) + m.Name[1:]
}

type contentModelResponse struct {
	Total int
	Skip  int
	Limit int
	Items []contentfulModel `json:"items"`
}

type contentfulLocale struct {
	Name         string `json:"name"`
	InternalCode string `json:"internal_code"`
	Code         string `json:"code"`
	Default      bool   `json:"default"`
}

type localeResponse struct {
	Total int
	Skip  int
	Limit int
	Items []contentfulLocale `json:"items"`
}

var (
	models  []contentfulModel
	locales = []string{}
	certs   string
)

// contentful api endpoints
const cdaEndpoint = "cdn.contentful.com"
const cmaEndpoint = "api.contentful.com"
const cpaEndpoint = "preview.contentful.com"

func fetchModels() ([]contentfulModel, error) {
	var url = fmt.Sprintf(
		"https://%s/spaces/%s/content_types?access_token=%s",
		cmaEndpoint,
		os.Getenv("CONTENTFUL_SPACE_ID"),
		os.Getenv("CONTENTFUL_AUTH_TOKEN"),
	)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	// TODO paginate model responses, if necessary
	var data contentModelResponse
	bs, _ := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(bs, &data); err != nil {
		return nil, err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}

	return data.Items, nil
}

func fetchLocales() ([]contentfulLocale, error) {
	var url = fmt.Sprintf(
		"https://%s/spaces/%s/locales",
		cmaEndpoint,
		os.Getenv("CONTENTFUL_SPACE_ID"),
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("CONTENTFUL_CMA_TOKEN")))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// TODO paginate model responses, if necessary
	var data localeResponse
	bs, _ := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(bs, &data); err != nil {
		return nil, err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}

	return data.Items, nil
}

func init() {
	var err error
	models, err = fetchModels()
	if err != nil {
		log.Fatal(err)
	}
	for i := range models {
		models[i].Name = strings.Replace(models[i].Name, " ", "", -1)
	}

	cfLocales, err := fetchLocales()
	if err != nil {
		log.Fatal(err)
	}
	for _, l := range cfLocales {
		locales = append(locales, l.Code)
	}

	fmt.Printf("%#v\n", locales)

	certs, err = fetchCerts()
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	var pkg string
	var output string
	flag.StringVar(&pkg, "pkg", "contentful", "package name")
	flag.StringVar(&output, "o", "contentful.go", "output file")
	flag.Parse()

	f := jen.NewFile(pkg)

	generateDateType(f)
	generateAssetType(f)
	generateResponseTypes(f)
	generateIteratorCacheType(f)
	for _, model := range models {
		generateModelType(f, model)
	}

	generateIteratorUtils(f)
	generateContentClient(f)
	generateManagementClient(f)

	file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal(err)
	}
	if err := f.Render(file); err != nil {
		log.Fatal(err)
	}
	file.Close()
}

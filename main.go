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

var (
	models []contentfulModel
	certs  string
)

// contentful api endpoints
const cdaEndpoint = "cdn.contentful.com"
const cmaEndpoint = "api.contentful.com"
const cpaEndpoint = "preview.contentful.com"

func init() {
	var url = fmt.Sprintf("https://%s/spaces/%s/environments/%s/content_types?access_token=%s", cmaEndpoint, os.Getenv("CONTENTFUL_SPACE_ID"), os.Getenv("CONTENTFUL_ENVIRONMENT"), os.Getenv("CONTENTFUL_AUTH_TOKEN"))
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	var data contentModelResponse
	bs, _ := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(bs, &data); err != nil {
		log.Fatal(err.Error())
	}
	if err := resp.Body.Close(); err != nil {
		log.Fatal(err.Error())
	}

	models = data.Items

	for i := range models {
		models[i].Name = strings.Replace(models[i].Name, " ", "", -1)
	}

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

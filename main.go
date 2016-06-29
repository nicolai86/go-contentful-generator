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

	. "github.com/davelondon/jennifer/jen"
)

type field struct {
	Name      string `json:"id"`
	Type      string `json:"type"`
	LinkType  string `json:"linkType"`
	Localized bool   `json:"localized"`
	Required  bool   `json:"required"`
	Disabled  bool   `json:"disabled"`
	Items     struct {
		Type        string `json:"type"`
		LinkType    string `json:"linkType"`
		Validations []struct {
			LinkContentType []string `json:"linkContentType"`
		} `json:"validations"`
	}
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
)

const endpoint = "https://cdn.contentful.com"

func init() {
	var url = fmt.Sprintf("%s/spaces/%s/content_types?access_token=%s", endpoint, os.Getenv("CONTENTFUL_SPACE_ID"), os.Getenv("CONTENTFUL_AUTH_TOKEN"))
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	var data contentModelResponse
	bs, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(bs, &data)
	resp.Body.Close()
	models = data.Items
}

func generateResponseTypes(f *File) {
	f.Type().Id("includes").Struct(
		Id("Entries").Index().Id("includeEntry").Tag(map[string]string{"json": "Entry"}),
		Id("Assets").Index().Id("includeAsset").Tag(map[string]string{"json": "Asset"}),
	)

	f.Type().Id("sys").Struct(
		Id("ID").String().Tag(map[string]string{"json": "id"}),
		Id("Type").String().Tag(map[string]string{"json": "type"}),
		Id("ContentType").Struct(
			Id("Sys").Struct(
				Id("ID").String().Tag(map[string]string{"json": "id"}),
			).Tag(map[string]string{"json": "sys"}),
		).Tag(map[string]string{"json": "contentType"}),
	)

	f.Type().Id("entryID").Struct(
		Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
	)
	f.Type().Id("entryIDs").Index().Id("entryID")

	f.Type().Id("includeEntry").Struct(
		Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
		Id("Fields").Op("*").Qual("encoding/json", "RawMessage").Tag(map[string]string{"json": "fields"}),
	)

	f.Type().Id("includeAsset").Struct(
		Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
		Id("Fields").Struct(
			Id("File").Struct(
				Id("URL").String().Tag(map[string]string{"json": "url"}),
				Id("Details").Struct(
					Id("Image").Struct(
						Id("Width").Int64().Tag(map[string]string{"json": "width"}),
						Id("Height").Int64().Tag(map[string]string{"json": "height"}),
					).Tag(map[string]string{"json": "image"}),
				).Tag(map[string]string{"json": "details"}),
			).Tag(map[string]string{"json": "file"}),
		).Tag(map[string]string{"json": "fields"}),
	)
}

func generateDateType(f *File) {
	f.Const().Id("dateLayout").Op("=").Lit("2006-01-02")
	f.Comment("Date defines an ISO 8601 date only time")
	f.Type().Id("Date").Qual("time", "Time")

	f.Func().Params(
		Id("d").Op("*").Id("Date"),
	).Id("UnmarshalJSON").Params(Id("b").Index().Byte()).Call(
		Id("error"),
	).Block(
		Id("s").Op(":=").Qual("strings", "Trim").Call(Id("string").Call(Id("b")), Lit("\"")),
		If(Id("s").Op("==").Lit("null")).Block(
			Op("*").Id("d").Op("=").Id("Date").Call(Qual("time", "Time").Block()),
		),
		List(Id("t"), Id("err")).Op(":=").Qual("time", "Parse").Call(Id("dateLayout"), Id("s")),
		Op("*").Id("d").Op("=").Id("Date").Call(Id("t")),
		Return(Id("err")),
	)
}

func generateAssetType(f *File) {
	f.Comment("Asset defines a media item in contentful")
	f.Type().Id("Asset").Struct(
		Id("Title").String(),
		Id("Description").String(),
		Id("URL").String(),
		Id("Width").Int64(),
		Id("Height").Int64(),
		Id("Size").Int64(),
	)
}

func generateModelResolvers(m contentfulModel, includes string) []Code {
	var parseSts = make([]Code, 0)
	for _, field := range m.Fields {
		fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
		if field.Type == "Symbol" || field.Type == "Text" {
			parseSts = append(parseSts, Id(fieldName).Op(":").Sel(Id("item"), Id("Fields"), Id(fieldName)).Op(","))
		} else if field.Type == "Integer" {
			parseSts = append(parseSts, Id(fieldName).Op(":").Sel(Id("item"), Id("Fields"), Id(fieldName)).Op(","))
		} else if field.Type == "Number" {
			parseSts = append(parseSts, Id(fieldName).Op(":").Sel(Id("item"), Id("Fields"), Id(fieldName)).Op(","))
		} else if field.Type == "Boolean" {
			parseSts = append(parseSts, Id(fieldName).Op(":").Sel(Id("item"), Id("Fields"), Id(fieldName)).Op(","))
		} else if field.Type == "Date" {
			parseSts = append(parseSts, Id(fieldName).Op(":").Sel(Id("item"), Id("Fields"), Id(fieldName)).Op(","))
		} else if field.Type == "Link" {
			if field.LinkType == "Asset" {
				parseSts = append(parseSts, Id(fieldName).Op(":").Id("resolveAsset").Params(Sel(Id("item"), Id("Fields"), Id(fieldName), Id("Sys"), Id("ID")), Id(includes)).Op(","))
			} else if field.LinkType == "Entry" {
				// FIXME if possible find singular type reference
				parseSts = append(parseSts, Id(fieldName).Op(":").Id("resolveEntry").Params(Sel(Id("item"), Id("Fields"), Id(fieldName)), Id(includes)).Op(","))
			}
		} else if field.Type == "Array" {
			if field.Items.Type == "Symbol" || field.Items.Type == "Text" {
				parseSts = append(parseSts, Id(fieldName).Op(":").Sel(Id("item"), Id("Fields"), Id(fieldName)).Op(","))
			} else if field.Items.Type == "Link" {
				var linkedTypes []string
				for _, validation := range field.Items.Validations {
					for _, linked := range validation.LinkContentType {
						for _, model := range models {
							if model.Sys.ID == linked {
								linkedTypes = append(linkedTypes, model.Name)
							}
						}
					}
				}

				// single type referenced, convert to typed array
				if len(linkedTypes) == 1 {
					targetName := linkedTypes[0]
					parseSts = append(parseSts, Id(fieldName).Op(":").Id(fmt.Sprintf("resolve%ss", targetName)).Params(Sel(Id("item"), Id("Fields"), Id(fieldName)), Id(includes)).Op(","))
				} else {
					parseSts = append(parseSts, Id(fieldName).Op(":").Id("resolveEntries").Params(Sel(Id("item"), Id("Fields"), Id(fieldName)), Id(includes)).Op(","))
				}
			}
		}
	}
	return parseSts
}

func generateModelItemAttributes(m contentfulModel) []Code {
	var payloadSts = make([]Code, 0)
	for _, field := range m.Fields {
		fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
		if field.Type == "Symbol" || field.Type == "Text" {
			payloadSts = append(payloadSts, Id(fieldName).String().Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Integer" {
			payloadSts = append(payloadSts, Id(fieldName).Int64().Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Number" {
			payloadSts = append(payloadSts, Id(fieldName).Float64().Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Boolean" {
			payloadSts = append(payloadSts, Id(fieldName).Bool().Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Date" {
			payloadSts = append(payloadSts, Id(fieldName).Id("Date").Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Link" {
			payloadSts = append(payloadSts, Id(fieldName).Id("entryID").Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Array" {
			if field.Items.Type == "Symbol" || field.Items.Type == "Text" {
				payloadSts = append(payloadSts, Id(fieldName).Index().String().Tag(map[string]string{"json": field.Name}))
			} else if field.Items.Type == "Link" {
				payloadSts = append(payloadSts, Id(fieldName).Id("entryIDs").Tag(map[string]string{"json": field.Name}))
			}
		}
	}
	return payloadSts
}

func generateModelAttributes(m contentfulModel) []Code {
	var sts = make([]Code, 0)
	for _, field := range m.Fields {
		fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
		if field.Type == "Symbol" || field.Type == "Text" {
			sts = append(sts, Id(fieldName).String())
		} else if field.Type == "Integer" {
			sts = append(sts, Id(fieldName).Int64())
		} else if field.Type == "Number" {
			sts = append(sts, Id(fieldName).Float64())
		} else if field.Type == "Boolean" {
			sts = append(sts, Id(fieldName).Bool())
		} else if field.Type == "Date" {
			sts = append(sts, Id(fieldName).Id("Date"))
		} else if field.Type == "Link" {
			if field.LinkType == "Asset" {
				sts = append(sts, Id(fieldName).Id("Asset"))
			} else if field.LinkType == "Entry" {
				// FIXME if possible find singular type reference
				sts = append(sts, Id(fieldName).Interface())
			}
		} else if field.Type == "Array" {
			if field.Items.Type == "Symbol" || field.Items.Type == "Text" {
				sts = append(sts, Id(fieldName).Index().String())
			} else if field.Items.Type == "Link" {
				var linkedTypes []string
				for _, validation := range field.Items.Validations {
					for _, linked := range validation.LinkContentType {
						for _, model := range models {
							if model.Sys.ID == linked {
								linkedTypes = append(linkedTypes, model.Name)
							}
						}
					}
				}

				// single type referenced, convert to typed array
				if len(linkedTypes) == 1 {
					sts = append(sts, Id(fieldName).Index().Id(linkedTypes[0]))
				} else {
					sts = append(sts, Id(fieldName).Index().Interface())
				}
			}
		}
	}
	return sts
}

func generateModelType(f *File, m contentfulModel) {
	f.Comment(fmt.Sprintf("%s %s", m.Name, m.Description))
	f.Type().Id(m.Name).Struct(generateModelAttributes(m)...)

	f.Comment(fmt.Sprintf("%sItem contains a single contentful %s model", m.DowncasedName(), m.Name))
	f.Type().Id(fmt.Sprintf("%sItem", m.DowncasedName())).Struct(
		Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
		Id("Fields").Struct(generateModelItemAttributes(m)...).Tag(map[string]string{"json": "fields"}),
	)

	f.Comment(fmt.Sprintf("%sResponse holds an entire contentful %s response", m.DowncasedName(), m.Name))
	f.Type().Id(fmt.Sprintf("%sResponse", m.DowncasedName())).Struct(
		Id("Items").Index().Id(fmt.Sprintf("%sItem", m.DowncasedName())).Tag(map[string]string{"json": "items"}),
		Id("Includes").Id("includes").Tag(map[string]string{"json": "includes"}),
	)

	f.Func().Id(fmt.Sprintf("resolve%s", m.Name)).Params(
		Id("entryID").String(),
		Id("includes").Id("includes"),
	).Id(m.Name).Block(
		Var().Id("item").Id(fmt.Sprintf("%sItem", m.DowncasedName())),
		For(List(Id("_"), Id("entry"))).Op(":=").Range().Id("includes.Entries").Block(
			If(Id("entry.Sys.ID").Op("==").Id("entryID")).Block(
				Qual("encoding/json", "Unmarshal").Params(Op("*").Id("entry.Fields"), Op("&").Id("item.Fields")),
				Return(Id(m.Name).Block(generateModelResolvers(m, "includes")...)),
			),
		),
		Return(Id(m.Name).Block()),
	)

	f.Func().Id(fmt.Sprintf("resolve%ss", m.Name)).Params(
		Id("ids").Id("entryIDs"),
		Id("includes").Id("includes"),
	).Index().Id(m.Name).Block(
		Var().Id("items").Index().Id(m.Name),
		Var().Id("item").Id(fmt.Sprintf("%sItem", m.DowncasedName())),

		For(List(Id("_"), Id("entry"))).Op(":=").Range().Id("includes.Entries").Block(
			Var().Id("included").Op("=").Lit(false),
			For(List(Id("_"), Id("entryID"))).Op(":=").Range().Id("ids").Block(
				Id("included").Op("=").Id("included").Op("||").Id("entryID.Sys.ID").Op("==").Id("entry.Sys.ID"),
			),
			If(Id("included").Op("==").Lit(true)).Block(
				Qual("encoding/json", "Unmarshal").Params(Op("*").Id("entry.Fields"), Op("&").Id("item.Fields")),
				Id("items").Op("=").Append(Id("items"), Id(m.Name).Block(generateModelResolvers(m, "includes")...)),
			),
		),
		Return(Id("items")),
	)

	f.Comment(fmt.Sprintf("Fetch%s retrieves paginated %ss", m.Name, m.Name))
	f.Func().Params(
		Id("c").Op("*").Id("Client"),
	).Id(fmt.Sprintf("Fetch%s", m.Name)).Params().Call(
		Index().Id(m.Name), Id("error"),
	).Block(
		Var().Id("url").Op("=").Qual("fmt", "Sprintf").Params(
			Lit("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=10&locale=%s"),
			Id("c.host"),
			Id("c.spaceID"),
			Id("c.authToken"),
			Lit(m.Sys.ID),
			Id("c.Locales").Index(Lit(0)),
		),
		List(Id("resp"), Id("err")).Op(":=").Id("c.client.Get").Params(Id("url")),
		If(Id("err").Op("!=").Id("nil")).Block(
			Return(Id("nil"), Id("err")),
		),
		If(Sel(Id("resp"), Id("StatusCode")).Op("!=").Qual("net/http", "StatusOK")).Block(
			Return(Id("nil"), Qual("fmt", "Errorf").Call(Lit("Request failed: %s, %v"), Id("resp.Status"), Id("err"))),
		),
		Var().Id("data").Id(fmt.Sprintf("%sResponse", m.DowncasedName())),
		If(
			Id("err").Op(":=").Sel(
				Qual("encoding/json", "NewDecoder").Params(Id("resp.Body")),
				Id("Decode").Params(Op("&").Id("data")),
			).Op(";").Id("err").Op("!=").Id("nil"),
		).Block(Return(Id("nil"), Id("err"))),
		Sel(
			Id("resp.Body.Close").Call(),
		),
		Var().Id("items").Op("=").Make(Index().Id(m.Name), Id("len").Call(Id("data.Items"))),
		For(List(Id("i"), Id("item"))).Op(":=").Range().Id("data.Items").Block(
			Id("items").Index(Id("i")).Op("=").Id(m.Name).Block(
				generateModelResolvers(m, "data.Includes")...,
			),
		),
		Return(Id("items"), Id("nil")),
	)
}

func generateClient(f *File) {

	// TODO return error if not resolvable
	f.Func().Id("resolveAsset").Params(
		Id("assetID").String(),
		Id("includes").Id("includes"),
	).Id("Asset").Block(
		For(List(Id("_"), Id("asset"))).Op(":=").Range().Id("includes.Assets").Block(
			If(Id("asset.Sys.ID").Op("==").Id("assetID")).Block(
				Return(Id("Asset").Block(
					Id("URL").Op(":").Qual("fmt", "Sprintf").Params(Lit("https:%s"), Id("asset.Fields.File.URL")).Op(","),
					Id("Width").Op(":").Id("asset.Fields.File.Details.Image.Width").Op(","),
					Id("Height").Op(":").Id("asset.Fields.File.Details.Image.Height").Op(","),
					Id("Size").Op(":").Lit(0).Op(","),
				)),
			),
		),
		Return(Id("Asset").Block()),
	)

	var sts = make([]Code, 0)
	for _, m := range models {
		sts = append(sts,
			If(Id("entry.Sys.ContentType.Sys.ID").Op("==").Lit(m.Sys.ID)).Block(
				Id("items").Op("=").Append(Id("items"), Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Params(Id("entry.Sys.ID"), Id("includes"))),
			),
		)
	}
	f.Func().Id("resolveEntries").Params(
		Id("ids").Id("entryIDs"),
		Id("includes").Id("includes"),
	).Index().Interface().Block(
		Var().Id("items").Index().Interface(),

		For(List(Id("_"), Id("entry"))).Op(":=").Range().Id("includes.Entries").Block(
			Var().Id("included").Op("=").Lit(false),
			For(List(Id("_"), Id("entryID"))).Op(":=").Range().Id("ids").Block(
				Id("included").Op("=").Id("included").Op("||").Id("entryID.Sys.ID").Op("==").Id("entry.Sys.ID"),
			),
			If(Id("included").Op("==").Lit(true)).Block(sts...,
			// TODO detect model
			// Var().Id("item").Id(fmt.Sprintf("%sItem", m.DowncasedName())),
			// Qual("encoding/json", "Unmarshal").Params(Op("*").Id("entry.Fields"), Op("&").Id("item.Fields")),
			// Id("items").Op("=").Append(Id("items"), Id(m.Name).Block(generateModelResolvers(m, "includes")...)),
			),
		),
		Return(Id("items")),
	)

	sts = make([]Code, 0)
	for _, m := range models {
		sts = append(sts,
			If(Id("entry.Sys.ContentType.Sys.ID").Op("==").Lit(m.Sys.ID)).Block(
				Return(Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Params(Id("entry.Sys.ID"), Id("includes"))),
			),
		)
	}
	f.Func().Id("resolveEntry").Params(
		Id("id").Id("entryID"),
		Id("includes").Id("includes"),
	).Interface().Block(
		For(List(Id("_"), Id("entry"))).Op(":=").Range().Id("includes.Entries").Block(
			If(Id("entry.Sys.ID").Op("==").Id("id.Sys.ID")).Block(sts...),
		),
		Return(Id("nil")),
	)

	f.Comment("Client")
	f.Type().Id("Client").Struct(
		Id("host").String(),
		Id("spaceID").String(),
		Id("authToken").String(),
		Id("Locales").Index().String(),
		Id("client").Op("*").Qual("net/http", "Client"),
	)

	// TODO include cert pool
	f.Comment("New returns a contentful client")
	f.Func().Id("New").Params(
		Id("authToken").String(),
		Id("locales").Index().String(),
	).Op("*").Id("Client").Block(
		Return(Op("&").Id("Client").Block(
			Id("host").Op(":").Lit("https://cdn.contentful.com").Op(","),
			Id("spaceID").Op(":").Lit(os.Getenv("CONTENTFUL_SPACE_ID")).Op(","),
			Id("authToken").Op(":").Id("authToken").Op(","),
			Id("Locales").Op(":").Id("locales").Op(","),
			Id("client").Op(":").Op("&").Qual("net/http", "Client").Block().Op(","),
		)),
	)
}

func main() {
	var pkg string
	var output string
	flag.StringVar(&pkg, "pkg", "contentful", "package name")
	flag.StringVar(&output, "o", "contentful.go", "output file")
	flag.Parse()

	f := NewFile(pkg)

	generateDateType(f)
	generateAssetType(f)
	generateResponseTypes(f)
	for _, model := range models {
		generateModelType(f, model)
	}

	generateClient(f)

	file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(file, "%#v", f)
	file.Close()
}

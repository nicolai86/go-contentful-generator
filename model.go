package main

import (
	"fmt"
	"strings"

	"github.com/davelondon/jennifer/jen"
	"github.com/gedex/inflector"
)

func linkedContentTypes(vs []validation) []string {
	var linkedTypes = []string{}
	for _, v := range vs {
		for _, linked := range v.LinkContentType {
			for _, model := range models {
				if model.Sys.ID == linked {
					linkedTypes = append(linkedTypes, model.Name)
				}
			}
		}
	}
	return linkedTypes
}

func generateModelResolvers(model contentfulModel, includes string) func(map[jen.Code]jen.Code) {
	return func(m map[jen.Code]jen.Code) {
		m[jen.Id("ID")] = jen.Id("item.Sys.ID")

		for _, field := range model.Fields {
			fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
			switch field.Type {
			case "Symbol", "Text", "Integer", "Number", "Boolean", "Date":
				m[jen.Id(fieldName)] = jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName))
			case "Link":
				switch field.LinkType {
				case "Asset":
					m[jen.Id(fieldName)] = jen.Id("resolveAsset").Call(
						jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName), jen.Id("Sys"), jen.Id("ID")),
						jen.Id(includes),
					)
				case "Entry":
					var linkedTypes = linkedContentTypes(field.Validations)

					// single type referenced, convert to typed array
					if len(linkedTypes) == 1 {
						if model.Name == linkedTypes[0] {
							// recursive types
							m[jen.Id(fieldName)] = jen.Id(fmt.Sprintf("resolve%sPtr", linkedTypes[0])).Call(
								jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName), jen.Id("Sys"), jen.Id("ID")),
								jen.Id(includes),
							)
						} else {
							m[jen.Id(fieldName)] = jen.Id(fmt.Sprintf("resolve%s", linkedTypes[0])).Call(
								jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName), jen.Id("Sys"), jen.Id("ID")),
								jen.Id(includes),
							)
						}
					} else {
						m[jen.Id(fieldName)] = jen.Id("resolveEntry").Call(
							jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)),
							jen.Id(includes),
						)
					}
				}
			case "Array":
				switch field.Items.Type {
				case "Symbol", "Text":
					m[jen.Id(fieldName)] = jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName))
				case "Link":
					var linkedTypes = linkedContentTypes(field.Items.Validations)

					// single type referenced, convert to typed array
					if len(linkedTypes) == 1 {
						targetName := linkedTypes[0]
						if targetName == model.Name {
							m[jen.Id(fieldName)] = jen.Id(fmt.Sprintf("resolve%ssPtr", targetName)).Call(
								jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)),
								jen.Id(includes),
							)
						} else {
							m[jen.Id(fieldName)] = jen.Id(fmt.Sprintf("resolve%ss", targetName)).Call(
								jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)),
								jen.Id(includes),
							)
						}
					} else {
						m[jen.Id(fieldName)] = jen.Id("resolveEntries").Call(
							jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)),
							jen.Id(includes),
						)
					}
				}
			}
		}
	}
}

func generateModelItemAttributes(m contentfulModel) func(*jen.Group) {
	return func(g *jen.Group) {
		for _, field := range m.Fields {
			fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
			switch field.Type {
			case "Symbol", "Text":
				g.Id(fieldName).String().Tag(map[string]string{"json": field.Name})
			case "Integer":
				g.Id(fieldName).Int64().Tag(map[string]string{"json": field.Name})
			case "Number":
				g.Id(fieldName).Float64().Tag(map[string]string{"json": field.Name})
			case "Boolean":
				g.Id(fieldName).Bool().Tag(map[string]string{"json": field.Name})
			case "Date":
				g.Id(fieldName).Id("Date").Tag(map[string]string{"json": field.Name})
			case "Link":
				g.Id(fieldName).Id("entryID").Tag(map[string]string{"json": field.Name})
			case "Array":
				switch field.Items.Type {
				case "Symbol", "Text":
					g.Id(fieldName).Index().String().Tag(map[string]string{"json": field.Name})
				case "Link":
					g.Id(fieldName).Id("entryIDs").Tag(map[string]string{"json": field.Name})
				}
			}
		}
	}
}

func generateModelAttributes(m contentfulModel) func(*jen.Group) {
	return func(g *jen.Group) {
		g.Id("ID").String()

		for _, field := range m.Fields {
			fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
			switch field.Type {
			case "Symbol", "Text":
				g.Id(fieldName).String()
			case "Integer":
				g.Id(fieldName).Int64()
			case "Number":
				g.Id(fieldName).Float64()
			case "Boolean":
				g.Id(fieldName).Bool()
			case "Date":
				g.Id(fieldName).Id("Date")
			case "Link":
				switch field.LinkType {
				case "Asset":
					g.Id(fieldName).Id("Asset")
				case "Entry":
					var linkedTypes = linkedContentTypes(field.Validations)

					// single type referenced, convert to typed array
					if len(linkedTypes) == 1 {
						if m.Name == linkedTypes[0] {
							// recursive type
							g.Id(fieldName).Op("*").Id(linkedTypes[0])
						} else {
							g.Id(fieldName).Id(linkedTypes[0])
						}
					} else {
						g.Id(fieldName).Interface()
					}
				}
			case "Array":
				switch field.Items.Type {
				case "Symbol", "Text":
					g.Id(fieldName).Index().String()
				case "Link":
					var linkedTypes = linkedContentTypes(field.Items.Validations)

					// single type referenced, convert to typed array
					if len(linkedTypes) == 1 {
						if m.Name == linkedTypes[0] {
							//  recursive types
							g.Id(fieldName).Index().Op("*").Id(linkedTypes[0])
						} else {
							g.Id(fieldName).Index().Id(linkedTypes[0])
						}
					} else {
						g.Id(fieldName).Index().Interface()
					}
				}
			}
		}
	}
}

func generateModelType(f *jen.File, m contentfulModel) {
	f.Commentf("%sIterator is used to paginate result sets of %s", m.Name, m.Name)
	f.Type().Id(fmt.Sprintf("%sIterator", m.Name)).Struct(
		jen.Id("Page").Int(),
		jen.Id("Limit").Int(),
		jen.Id("Offset").Int(),
		jen.Id("IncludeCount").Int(),
		jen.Id("c").Op("*").Id("Client"),
		jen.Id("items").Index().Op("*").Id(m.Name),
	)

	f.Commentf("Next returns the following item of type %s. If none exists a network request will be executed", m.Name)
	f.Func().Params(
		jen.Id("it").Op("*").Id(fmt.Sprintf("%sIterator", m.Name)),
	).Id("Next").Params().Params(
		jen.Op("*").Id(m.Name), jen.Id("error"),
	).Block(
		jen.If(jen.Len(jen.Id("it.items")).Op("==").Lit(0)).Block(
			jen.If(
				jen.Err().Op(":=").Id("it.fetch").Call(),
				jen.Err().Op("!=").Nil(),
			).Block(
				jen.Return(jen.Nil(), jen.Err()),
			),
		),
		jen.If(jen.Len(jen.Id("it.items")).Op("==").Lit(0)).Block(
			jen.Return(jen.Nil(), jen.Id("IteratorDone")),
		),
		jen.Var().Id("item").Op("*").Id(m.Name),
		jen.List(
			jen.Id("item"),
			jen.Id("it.items"),
		).Op("=").List(
			jen.Id("it.items").Index(jen.Len(jen.Id("it.items")).Op("-").Lit(1)),
			jen.Id("it.items").Index(jen.Empty(), jen.Len(jen.Id("it.items")).Op("-").Lit(1)),
		),
		jen.If(jen.Len(jen.Id("it.items")).Op("==").Lit(0)).Block(
			jen.Id("it.Page").Op("++"),
			jen.Id("it.Offset").Op("=").Id("it.Page").Op("*").Id("it.Limit"),
		),
		jen.Return(jen.Id("item, nil")),
	)

	f.Func().Params(
		jen.Id("it").Op("*").Id(fmt.Sprintf("%sIterator", m.Name)),
	).Id("fetch").Params().Id("error").Block(
		jen.Id("c").Op(":=").Id("it.c"),
		jen.Var().Id("url").Op("=").Qual("fmt", "Sprintf").Params(
			jen.Lit("%s/spaces/%s/entries?access_token=%s&content_type=%s&include=%d&locale=%s&limit=%d&skip=%d"),
			jen.Id("c.host"),
			jen.Id("c.spaceID"),
			jen.Id("c.authToken"),
			jen.Lit(m.Sys.ID),
			jen.Id("it.IncludeCount"),
			jen.Id("c.Locales").Index(jen.Lit(0)),
			jen.Id("it.Limit"),
			jen.Id("it.Offset"),
		),
		jen.List(jen.Id("resp"), jen.Err()).Op(":=").Id("c.client.Get").Call(jen.Id("url")),
		jen.If(jen.Err().Op("!=").Nil()).Block(
			jen.Return(jen.Err()),
		),
		jen.If(jen.Sel(jen.Id("resp"), jen.Id("StatusCode")).Op("!=").Qual("net/http", "StatusOK")).Block(
			jen.Return(
				jen.Qual("fmt", "Errorf").Call(
					jen.Lit("Request failed: %s, %v"),
					jen.Id("resp.Status"),
					jen.Err(),
				),
			),
		),
		jen.Var().Id("data").Id(fmt.Sprintf("%sResponse", m.DowncasedName())),
		jen.If(
			jen.Err().Op(":=").Sel(
				jen.Qual("encoding/json", "NewDecoder").Call(jen.Id("resp.Body")),
				jen.Id("Decode").Call(jen.Op("&").Id("data")),
			),
			jen.Err().Op("!=").Nil(),
		).Block(jen.Return(jen.Err())),
		jen.If(
			jen.Err().Op(":=").Sel(
				jen.Id("resp"),
				jen.Id("Body"),
				jen.Id("Close").Call(),
			),
			jen.Err().Op("!=").Nil(),
		).Block(
			jen.Return(jen.Err()),
		),
		jen.Var().Id("items").Op("=").Make(jen.Index().Op("*").Id(m.Name), jen.Len(jen.Id("data.Items"))),
		jen.For(jen.List(jen.Id("i"), jen.Id("item")).Op(":=").Range().Id("data.Items")).Block(
			jen.Id("items").Index(jen.Id("i")).Op("=").Op("&").Id(m.Name).DictFunc(
				generateModelResolvers(m, "data.Includes"),
			),
		),
		jen.Id("it.items").Op("=").Id("items"),
		jen.Return(jen.Nil()),
	)

	f.Commentf("%s %s", m.Name, m.Description)
	f.Type().Id(m.Name).StructFunc(generateModelAttributes(m))

	f.Commentf("%sItem contains a single contentful %s model", m.DowncasedName(), m.Name)
	f.Type().Id(fmt.Sprintf("%sItem", m.DowncasedName())).Struct(
		jen.Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
		jen.Id("Fields").StructFunc(generateModelItemAttributes(m)).Tag(map[string]string{"json": "fields"}),
	)

	f.Commentf("%sResponse holds an entire contentful %s response", m.DowncasedName(), m.Name)
	f.Type().Id(fmt.Sprintf("%sResponse", m.DowncasedName())).Struct(
		jen.Id("Total").Int().Tag(map[string]string{"json": "total"}),
		jen.Id("Skip").Int().Tag(map[string]string{"json": "skip"}),
		jen.Id("Limit").Int().Tag(map[string]string{"json": "limit"}),
		jen.Id("Items").Index().Id(fmt.Sprintf("%sItem", m.DowncasedName())).Tag(map[string]string{"json": "items"}),
		jen.Id("Includes").Id("includes").Tag(map[string]string{"json": "includes"}),
	)

	f.Func().Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Params(
		jen.Id("entryID").String(),
		jen.Id("includes").Id("includes"),
	).Id(m.Name).Block(
		jen.Var().Id("item").Id(fmt.Sprintf("%sItem", m.DowncasedName())),
		jen.For(jen.List(jen.Id("_"), jen.Id("entry")).Op(":=").Range().Id("includes.Entries")).Block(
			jen.If(jen.Id("entry.Sys.ID").Op("==").Id("entryID")).Block(
				jen.If(
					jen.Err().Op(":=").Qual("encoding/json", "Unmarshal").Call(
						jen.Op("*").Id("entry.Fields"),
						jen.Op("&").Id("item.Fields"),
					),
					jen.Err().Op("!=").Nil(),
				).Block(
					jen.Return(jen.Id(m.Name).Dict(nil)),
				),
				jen.Return(jen.Id(m.Name).DictFunc(generateModelResolvers(m, "includes"))),
			),
		),
		jen.Return(jen.Id(m.Name).Dict(nil)),
	)

	f.Func().Id(fmt.Sprintf("resolve%sPtr", m.CapitalizedName())).Params(
		jen.Id("entryID").String(),
		jen.Id("includes").Id("includes"),
	).Op("*").Id(m.Name).Block(
		jen.Var().Id("item").Op("=").Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Call(
			jen.Id("entryID"),
			jen.Id("includes"),
		),
		jen.Return(jen.Op("&").Id("item")),
	)

	f.Func().Id(fmt.Sprintf("resolve%ssPtr", m.CapitalizedName())).Params(
		jen.Id("ids").Id("entryIDs"),
		jen.Id("includes").Id("includes"),
	).Index().Op("*").Id(m.Name).Block(
		jen.Var().Id("items").Op("=").Id(fmt.Sprintf("resolve%ss", m.CapitalizedName())).Call(
			jen.Id("ids"),
			jen.Id("includes"),
		),
		jen.Var().Id("ptrs").Index().Op("*").Id(m.Name),
		jen.For(jen.List(jen.Id("_"), jen.Id("entry")).Op(":=").Range().Id("items")).Block(
			jen.Id("ptrs").Op("=").Append(jen.Id("ptrs"), jen.Op("&").Id("entry")),
		),
		jen.Return(jen.Id("ptrs")),
	)

	f.Func().Id(fmt.Sprintf("resolve%ss", m.CapitalizedName())).Params(
		jen.Id("ids").Id("entryIDs"),
		jen.Id("includes").Id("includes"),
	).Index().Id(m.Name).Block(
		jen.Var().Id("items").Index().Id(m.Name),
		jen.Var().Id("item").Id(fmt.Sprintf("%sItem", m.DowncasedName())),

		jen.For(jen.List(jen.Id("_"), jen.Id("entry")).Op(":=").Range().Id("includes.Entries")).Block(
			jen.Var().Id("included").Op("=").Lit(false),
			jen.For(jen.List(jen.Id("_"), jen.Id("entryID")).Op(":=").Range().Id("ids")).Block(
				jen.Id("included").Op("=").Id("included").Op("||").Id("entryID.Sys.ID").Op("==").Id("entry.Sys.ID"),
			),
			jen.If(jen.Id("included").Op("==").Lit(true)).Block(
				jen.If(
					jen.Err().Op(":=").Qual("encoding/json", "Unmarshal").Call(
						jen.Op("*").Id("entry.Fields"),
						jen.Op("&").Id("item.Fields"),
					),
					jen.Err().Op("!=").Nil(),
				).Block(
					jen.Return(jen.Id("items")),
				),
				jen.Id("items").Op("=").Append(
					jen.Id("items"),
					jen.Id(m.Name).DictFunc(generateModelResolvers(m, "includes")),
				),
			),
		),
		jen.Return(jen.Id("items")),
	)

	resolverName := inflector.Pluralize(m.Name)
	f.Commentf("%s retrieves paginated %s entries", resolverName, m.Name)
	f.Func().Params(
		jen.Id("c").Op("*").Id("Client"),
	).Id(resolverName).Params(
		jen.Id("opts").Id("ListOptions"),
	).Op("*").Id(fmt.Sprintf("%sIterator", m.Name)).Block(
		jen.If(jen.Id("opts.Limit").Op("<=").Lit(0)).Block(
			jen.Id("opts.Limit").Op("=").Lit(100),
		),
		jen.Id("it").Op(":=").Op("&").Id(fmt.Sprintf("%sIterator", m.Name)).Dict(map[jen.Code]jen.Code{
			jen.Id("Page"):         jen.Id("opts.Page"),
			jen.Id("Limit"):        jen.Id("opts.Limit"),
			jen.Id("IncludeCount"): jen.Id("opts.IncludeCount"),
			jen.Id("c"):            jen.Id("c"),
		}),
		jen.Return(jen.Id("it")),
	)
}

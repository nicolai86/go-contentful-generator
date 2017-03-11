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

func generateModelLinkResolver(model contentfulModel, items, includes, cache string) func(jen.Dict) {
	return func(d jen.Dict) {
		for _, field := range model.Fields {
			fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
			switch field.Type {
			case "Link":
				switch field.LinkType {
				case "Asset":
					d[jen.Id(fieldName)] = jen.Id("resolveAsset").Call(
						jen.Id("item").Dot("Fields").Dot(fieldName).Dot("Sys").Dot("ID"),
						jen.Id(includes),
					)
				case "Entry":
					var linkedTypes = linkedContentTypes(field.Validations)
					// single type referenced, convert to typed array
					if len(linkedTypes) == 1 {
						if model.Name == linkedTypes[0] {
							// 1:1 recursive type relationship
							d[jen.Id(fieldName)] = jen.Id(fmt.Sprintf("resolve%sPtr", linkedTypes[0])).Call(
								jen.Id("item").Dot("Fields").Dot(fieldName).Dot("Sys").Dot("ID"),
								jen.Id(items),
								jen.Id(includes),
								jen.Id(cache),
							)
						} else {
							// 1:1 type relationship
							d[jen.Id(fieldName)] = jen.Id(fmt.Sprintf("resolve%s", linkedTypes[0])).Call(
								jen.Id("item").Dot("Fields").Dot(fieldName).Dot("Sys").Dot("ID"),
								jen.Id(items),
								jen.Id(includes),
								jen.Id(cache),
							)
						}
					} else {
						// 1:1 multi-type relationship
						d[jen.Id(fieldName)] = jen.Id("resolveEntry").Call(
							jen.Id("item").Dot("Fields").Dot(fieldName),
							jen.Id(items),
							jen.Id(includes),
							jen.Id(cache),
						)
					}
				}
			case "Array":
				switch field.Items.Type {
				case "Link":
					var linkedTypes = linkedContentTypes(field.Items.Validations)

					// single type referenced, convert to typed array
					if len(linkedTypes) == 1 {
						if linkedTypes[0] == model.Name {
							// 1:N recursive type relationship
							d[jen.Id(fieldName)] = jen.Id(fmt.Sprintf("resolve%ssPtr", linkedTypes[0])).Call(
								jen.Id("item").Dot("Fields").Dot(fieldName),
								jen.Id(items),
								jen.Id(includes),
								jen.Id(cache),
							)
						} else {
							// 1:N type relationship
							d[jen.Id(fieldName)] = jen.Id(fmt.Sprintf("resolve%ss", linkedTypes[0])).Call(
								jen.Id("item").Dot("Fields").Dot(fieldName),
								jen.Id(items),
								jen.Id(includes),
								jen.Id(cache),
							)
						}
					} else {
						// 1:N multi-type relationship
						d[jen.Id(fieldName)] = jen.Id("resolveEntries").Call(
							jen.Id("item").Dot("Fields").Dot(fieldName),
							jen.Id(items),
							jen.Id(includes),
							jen.Id(cache),
						)
					}
				}
			}
		}
	}
}

func generateModelResolvers(model contentfulModel, items, includes, cache string, includeResolvers bool) jen.Dict {
	d := jen.Dict{
		jen.Id("ID"): jen.Id("item.Sys.ID"),
	}

	if includeResolvers {
		generateModelLinkResolver(model, items, includes, cache)(d)
	}

	for _, field := range model.Fields {
		fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
		switch field.Type {
		case "Symbol", "Text", "Integer", "Number", "Boolean", "Date":
			d[jen.Id(fieldName)] = jen.Id("item").Dot("Fields").Dot(fieldName)
		}
	}
	return d
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
							// 1:1 recursive type relationship
							g.Id(fieldName).Op("*").Id(linkedTypes[0])
						} else {
							// 1:1 type relationship
							g.Id(fieldName).Id(linkedTypes[0])
						}
					} else {
						// 1:1 multi-type relationship
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
							// 1:N recursive type relationship
							g.Id(fieldName).Index().Op("*").Id(linkedTypes[0])
						} else {
							// 1:N type relationship
							g.Id(fieldName).Index().Id(linkedTypes[0])
						}
					} else {
						// 1:N multi-type relationship
						g.Id(fieldName).Index().Interface()
					}
				}
			}
		}
	}
}

func generateIteratorCacheType(f *jen.File) {
	f.Type().Id("iteratorCache").StructFunc(func(g *jen.Group) {
		for _, m := range models {
			g.Id(fmt.Sprintf("%ss", m.DowncasedName())).Map(jen.String()).Op("*").Id(m.Name)
		}
	})
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
		jen.Id("lookupCache").Op("*").Id("iteratorCache"),
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
		jen.If(jen.Id("resp").Dot("StatusCode").Op("!=").Qual("net/http", "StatusOK")).Block(
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
			jen.Err().Op(":=").Qual("encoding/json", "NewDecoder").Call(
				jen.Id("resp.Body"),
			).Dot("Decode").Call(jen.Op("&").Id("data")),
			jen.Err().Op("!=").Nil(),
		).Block(jen.Return(jen.Err())),
		jen.If(
			jen.Err().Op(":=").Id("resp").Dot("Body").Dot("Close").Call(),
			jen.Err().Op("!=").Nil(),
		).Block(
			jen.Return(jen.Err()),
		),
		jen.Var().Id("items").Op("=").Make(jen.Index().Op("*").Id(m.Name), jen.Len(jen.Id("data.Items"))),
		jen.For(jen.List(jen.Id("i"), jen.Id("raw")).Op(":=").Range().Id("data.Items")).Block(
			jen.Var().Id("item").Id(fmt.Sprintf("%sItem", m.DowncasedName())),
			jen.If(
				jen.Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Call(
					jen.Id("*raw.Fields"),
					jen.Id("&item.Fields"),
				),
				jen.Err().Op("!=").Nil(),
			).Block(jen.Return(jen.Err())),
			jen.Id("items").Index(jen.Id("i")).Op("=").Op("&").Id(m.Name).Values(
				generateModelResolvers(m, "data.Items", "data.Includes", "it.lookupCache", true),
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
		jen.Id("Items").Index().Id("includeEntry").Tag(map[string]string{"json": "items"}),
		jen.Id("Includes").Id("includes").Tag(map[string]string{"json": "includes"}),
	)

	var codes = []jen.Code{}
	var attrs = jen.Dict{}
	generateModelLinkResolver(m, "items", "includes", "cache")(attrs)
	for k, v := range attrs {
		codes = append(codes, jen.Id("tmp").Op(".").Add(k).Op("=").Add(v).Op(";"))
	}

	var codess = []jen.Code{}
	var attrss = jen.Dict{}
	generateModelLinkResolver(m, "its", "includes", "cache")(attrss)
	for k, v := range attrss {
		codess = append(codess, jen.Id("tmp").Op(".").Add(k).Op("=").Add(v).Op(";"))
	}

	f.Func().Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Params(
		jen.Id("entryID").String(),
		jen.Id("items").Index().Id("includeEntry"),
		jen.Id("includes").Id("includes"),
		jen.Id("cache").Id("*iteratorCache"),
	).Id(m.Name).Block(
		jen.If(
			jen.Id("v, ok").Op(":=").Id(fmt.Sprintf("cache.%ss", m.DowncasedName())).Index(jen.Id("entryID")),
			jen.Id("ok"),
		).Block(
			jen.Return(jen.Op("*").Id("v")),
		),
		jen.Var().Id("item").Id(fmt.Sprintf("%sItem", m.DowncasedName())),
		jen.For(jen.List(jen.Id("_"), jen.Id("entry")).Op(":=").Range().Append(jen.Id("includes.Entries"), jen.Id("items..."))).Block(
			jen.If(jen.Id("entry.Sys.ID").Op("==").Id("entryID")).Block(
				jen.If(
					jen.Err().Op(":=").Qual("encoding/json", "Unmarshal").Call(
						jen.Op("*").Id("entry.Fields"),
						jen.Op("&").Id("item.Fields"),
					),
					jen.Err().Op("!=").Nil(),
				).Block(
					jen.Return(jen.Id(m.Name).Values()),
				),
				jen.Var().Id("tmp").Op("=").Op("&").Id(m.Name).Values(generateModelResolvers(m, "items", "includes", "cache", false)),
				jen.Id(fmt.Sprintf("cache.%ss", m.DowncasedName())).Index(jen.Id("entry.Sys.ID")).Op("=").Id("tmp"),
				jen.Add(codes...),
				jen.Return(jen.Op("*").Id("tmp")),
			),
		),
		jen.Return(jen.Id(m.Name).Values()),
	)

	f.Func().Id(fmt.Sprintf("resolve%sPtr", m.CapitalizedName())).Params(
		jen.Id("entryID").String(),
		jen.Id("items").Index().Id("includeEntry"),
		jen.Id("includes").Id("includes"),
		jen.Id("cache").Id("*iteratorCache"),
	).Op("*").Id(m.Name).Block(
		jen.Var().Id("item").Op("=").Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Call(
			jen.Id("entryID"),
			jen.Id("items"),
			jen.Id("includes"),
			jen.Id("cache"),
		),
		jen.Return(jen.Op("&").Id("item")),
	)

	f.Func().Id(fmt.Sprintf("resolve%ssPtr", m.CapitalizedName())).Params(
		jen.Id("ids").Id("entryIDs"),
		jen.Id("its").Index().Id("includeEntry"),
		jen.Id("includes").Id("includes"),
		jen.Id("cache").Id("*iteratorCache"),
	).Index().Op("*").Id(m.Name).Block(
		jen.Var().Id("items").Op("=").Id(fmt.Sprintf("resolve%ss", m.CapitalizedName())).Call(
			jen.Id("ids"),
			jen.Id("its"),
			jen.Id("includes"),
			jen.Id("cache"),
		),
		jen.Var().Id("ptrs").Index().Op("*").Id(m.Name),
		jen.For(jen.List(jen.Id("_"), jen.Id("entry")).Op(":=").Range().Id("items")).Block(
			jen.Id("ptrs").Op("=").Append(jen.Id("ptrs"), jen.Op("&").Id("entry")),
		),
		jen.Return(jen.Id("ptrs")),
	)

	f.Func().Id(fmt.Sprintf("resolve%ss", m.CapitalizedName())).Params(
		jen.Id("ids").Id("entryIDs"),
		jen.Id("its").Index().Id("includeEntry"),
		jen.Id("includes").Id("includes"),
		jen.Id("cache").Id("*iteratorCache"),
	).Index().Id(m.Name).Block(
		jen.Var().Id("items").Index().Id(m.Name),
		jen.For(jen.List(jen.Id("_"), jen.Id("entry")).Op(":=").Range().Append(jen.Id("includes.Entries"), jen.Id("its..."))).Block(
			jen.Var().Id("item").Id(fmt.Sprintf("%sItem", m.DowncasedName())),
			jen.Var().Id("included").Op("=").Lit(false),
			jen.For(jen.List(jen.Id("_"), jen.Id("entryID")).Op(":=").Range().Id("ids")).Block(
				jen.Id("included").Op("=").Id("included").Op("||").Id("entryID.Sys.ID").Op("==").Id("entry.Sys.ID"),
			),
			jen.If(jen.Id("included").Op("==").Lit(true)).Block(
				jen.If(
					jen.Id("v, ok").Op(":=").Id(fmt.Sprintf("cache.%ss", m.DowncasedName())).Index(jen.Id("entry.Sys.ID")),
					jen.Id("ok"),
				).Block(
					jen.Id("items").Op("=").Append(
						jen.Id("items"),
						jen.Op("*").Id("v"),
					),
					jen.Continue(),
				),

				jen.If(
					jen.Err().Op(":=").Qual("encoding/json", "Unmarshal").Call(
						jen.Op("*").Id("entry.Fields"),
						jen.Op("&").Id("item.Fields"),
					),
					jen.Err().Op("!=").Nil(),
				).Block(
					jen.Return(jen.Id("items")),
				),

				jen.Var().Id("tmp").Op("=").Op("&").Id(m.Name).Values(generateModelResolvers(m, "its", "includes", "cache", false)),
				jen.Id(fmt.Sprintf("cache.%ss", m.DowncasedName())).Index(jen.Id("entry.Sys.ID")).Op("=").Id("tmp"),
				jen.Add(codess...),
				jen.Id("items").Op("=").Append(
					jen.Id("items"),
					jen.Id("*tmp"),
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

		jen.Id("it").Op(":=").Op("&").Id(fmt.Sprintf("%sIterator", m.Name)).Values(jen.Dict{
			jen.Id("Page"):         jen.Id("opts.Page"),
			jen.Id("Limit"):        jen.Id("opts.Limit"),
			jen.Id("IncludeCount"): jen.Id("opts.IncludeCount"),
			jen.Id("c"):            jen.Id("c"),
			jen.Id("lookupCache"): jen.Op("&").Id("iteratorCache").Values(jen.DictFunc(func(d jen.Dict) {
				for _, m := range models {
					d[jen.Id(fmt.Sprintf("%ss", m.DowncasedName()))] = jen.Make(jen.Map(jen.String()).Op("*").Id(m.Name))
				}
			})),
		}),
		jen.Return(jen.Id("it")),
	)
}

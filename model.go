package main

import (
	"fmt"
	"strings"

	"github.com/davelondon/jennifer/jen"
	"github.com/gedex/inflector"
)

func generateModelResolvers(m contentfulModel, includes string) []jen.Code {
	var parseSts = []jen.Code{
		jen.Id("ID").Op(":").Id("item.Sys.ID").Op(","),
	}
	for _, field := range m.Fields {
		fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
		if field.Type == "Symbol" || field.Type == "Text" {
			parseSts = append(parseSts, jen.Id(fieldName).Op(":").Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)).Op(","))
		} else if field.Type == "Integer" {
			parseSts = append(parseSts, jen.Id(fieldName).Op(":").Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)).Op(","))
		} else if field.Type == "Number" {
			parseSts = append(parseSts, jen.Id(fieldName).Op(":").Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)).Op(","))
		} else if field.Type == "Boolean" {
			parseSts = append(parseSts, jen.Id(fieldName).Op(":").Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)).Op(","))
		} else if field.Type == "Date" {
			parseSts = append(parseSts, jen.Id(fieldName).Op(":").Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)).Op(","))
		} else if field.Type == "Link" {
			if field.LinkType == "Asset" {
				parseSts = append(parseSts, jen.Id(fieldName).Op(":").Id("resolveAsset").Params(jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName), jen.Id("Sys"), jen.Id("ID")), jen.Id(includes)).Op(","))
			} else if field.LinkType == "Entry" {
				var linkedTypes []string
				for _, validation := range field.Validations {
					for _, linked := range validation.LinkContentType {
						for _, model := range models {
							if model.Sys.ID == linked {
								linkedTypes = append(linkedTypes, model.Name)
							}
						}
					}
				}

				// single type referenced, convert to typed array
				// FIXME recursive types
				if len(linkedTypes) == 1 && m.Name != linkedTypes[0] {
					parseSts = append(parseSts, jen.Id(fieldName).Op(":").Id(fmt.Sprintf("resolve%s", linkedTypes[0])).Params(
						jen.Sel(
							jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName), jen.Id("Sys"), jen.Id("ID"),
						), jen.Id(includes),
					).Op(","))
				} else {
					parseSts = append(parseSts, jen.Id(fieldName).Op(":").Id("resolveEntry").Params(jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)), jen.Id(includes)).Op(","))
				}

			}
		} else if field.Type == "Array" {
			if field.Items.Type == "Symbol" || field.Items.Type == "Text" {
				parseSts = append(parseSts, jen.Id(fieldName).Op(":").Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)).Op(","))
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
				// FIXME recursive types
				if len(linkedTypes) == 1 && m.Name != linkedTypes[0] {
					targetName := linkedTypes[0]
					parseSts = append(parseSts, jen.Id(fieldName).Op(":").Id(fmt.Sprintf("resolve%ss", targetName)).Params(jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)), jen.Id(includes)).Op(","))
				} else {
					parseSts = append(parseSts, jen.Id(fieldName).Op(":").Id("resolveEntries").Params(jen.Sel(jen.Id("item"), jen.Id("Fields"), jen.Id(fieldName)), jen.Id(includes)).Op(","))
				}
			}
		}
	}
	return parseSts
}

func generateModelItemAttributes(m contentfulModel) []jen.Code {
	var payloadSts = make([]jen.Code, 0)
	for _, field := range m.Fields {
		fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
		if field.Type == "Symbol" || field.Type == "Text" {
			payloadSts = append(payloadSts, jen.Id(fieldName).String().Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Integer" {
			payloadSts = append(payloadSts, jen.Id(fieldName).Int64().Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Number" {
			payloadSts = append(payloadSts, jen.Id(fieldName).Float64().Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Boolean" {
			payloadSts = append(payloadSts, jen.Id(fieldName).Bool().Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Date" {
			payloadSts = append(payloadSts, jen.Id(fieldName).Id("Date").Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Link" {
			payloadSts = append(payloadSts, jen.Id(fieldName).Id("entryID").Tag(map[string]string{"json": field.Name}))
		} else if field.Type == "Array" {
			if field.Items.Type == "Symbol" || field.Items.Type == "Text" {
				payloadSts = append(payloadSts, jen.Id(fieldName).Index().String().Tag(map[string]string{"json": field.Name}))
			} else if field.Items.Type == "Link" {
				payloadSts = append(payloadSts, jen.Id(fieldName).Id("entryIDs").Tag(map[string]string{"json": field.Name}))
			}
		}
	}
	return payloadSts
}

func generateModelAttributes(m contentfulModel) []jen.Code {
	var sts = []jen.Code{
		jen.Id("ID").String(),
	}

	for _, field := range m.Fields {
		fieldName := strings.ToUpper(field.Name[0:1]) + field.Name[1:]
		if field.Type == "Symbol" || field.Type == "Text" {
			sts = append(sts, jen.Id(fieldName).String())
		} else if field.Type == "Integer" {
			sts = append(sts, jen.Id(fieldName).Int64())
		} else if field.Type == "Number" {
			sts = append(sts, jen.Id(fieldName).Float64())
		} else if field.Type == "Boolean" {
			sts = append(sts, jen.Id(fieldName).Bool())
		} else if field.Type == "Date" {
			sts = append(sts, jen.Id(fieldName).Id("Date"))
		} else if field.Type == "Link" {
			if field.LinkType == "Asset" {
				sts = append(sts, jen.Id(fieldName).Id("Asset"))
			} else if field.LinkType == "Entry" {
				var linkedTypes []string
				for _, validation := range field.Validations {
					for _, linked := range validation.LinkContentType {
						for _, model := range models {
							if model.Sys.ID == linked {
								linkedTypes = append(linkedTypes, model.Name)
							}
						}
					}
				}

				// single type referenced, convert to typed array
				// FIXME recursive types
				if len(linkedTypes) == 1 && m.Name != linkedTypes[0] {
					sts = append(sts, jen.Id(fieldName).Id(linkedTypes[0]))
				} else {
					sts = append(sts, jen.Id(fieldName).Interface())
				}
			}
		} else if field.Type == "Array" {
			if field.Items.Type == "Symbol" || field.Items.Type == "Text" {
				sts = append(sts, jen.Id(fieldName).Index().String())
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
				// FIXME recursive types
				if len(linkedTypes) == 1 && m.Name != linkedTypes[0] {
					sts = append(sts, jen.Id(fieldName).Index().Id(linkedTypes[0]))
				} else {
					sts = append(sts, jen.Id(fieldName).Index().Interface())
				}
			}
		}
	}
	return sts
}

func generateModelType(f *jen.File, m contentfulModel) {
	f.Comment(fmt.Sprintf("%sIterator is used to paginate result sets of %s", m.Name, m.Name))
	f.Type().Id(fmt.Sprintf("%sIterator", m.Name)).Struct(
		jen.Id("Page").Int(),
		jen.Id("Limit").Int(),
		jen.Id("Offset").Int(),
		jen.Id("IncludeCount").Int(),
		jen.Id("c").Op("*").Id("Client"),
		jen.Id("items").Index().Op("*").Id(m.Name),
	)

	f.Comment(fmt.Sprintf("Next returns the following item of type %s. If none exists a network request will be executed", m.Name))
	f.Func().Params(
		jen.Id("it").Op("*").Id(fmt.Sprintf("%sIterator", m.Name)),
	).Id("Next").Params().Params(
		jen.Op("*").Id(m.Name), jen.Id("error"),
	).Block(
		jen.If(jen.Len(jen.Id("it.items")).Op("==").Lit(0)).Block(
			jen.If(jen.Id("err").Op(":=").Id("it.fetch();").Id("err").Op("!=").Id("nil")).Block(
				jen.Return(jen.Id("nil"), jen.Id("err")),
			),
		),
		jen.If(jen.Len(jen.Id("it.items")).Op("==").Lit(0)).Block(
			jen.Return(jen.Id("nil"), jen.Id("IteratorDone")),
		),
		jen.Var().Id("item").Op("*").Id(m.Name),
		jen.Id("item, it.items").Op("=").Id("it.items[len(it.items)-1], it.items[:len(it.items)-1]"),
		jen.If(jen.Len(jen.Id("it.items")).Op("==").Lit(0)).Block(
			jen.Id("it.Page++"),
			jen.Id("it.Offset = it.Page * it.Limit"),
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
		jen.List(jen.Id("resp"), jen.Id("err")).Op(":=").Id("c.client.Get").Params(jen.Id("url")),
		jen.If(jen.Id("err").Op("!=").Id("nil")).Block(
			jen.Return(jen.Id("err")),
		),
		jen.If(jen.Sel(jen.Id("resp"), jen.Id("StatusCode")).Op("!=").Qual("net/http", "StatusOK")).Block(
			jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("Request failed: %s, %v"), jen.Id("resp.Status"), jen.Id("err"))),
		),
		jen.Var().Id("data").Id(fmt.Sprintf("%sResponse", m.DowncasedName())),
		jen.If(
			jen.Id("err").Op(":=").Sel(
				jen.Qual("encoding/json", "NewDecoder").Params(jen.Id("resp.Body")),
				jen.Id("Decode").Params(jen.Op("&").Id("data")),
			).Op(";").Id("err").Op("!=").Id("nil"),
		).Block(jen.Return(jen.Id("err"))),
		jen.If(
			jen.Id("err").Op(":=").Sel(
				jen.Id("resp"),
				jen.Id("Body"),
				jen.Id("Close").Call(),
			).Op(";").Id("err").Op("!=").Id("nil").Block(
				jen.Return(jen.Id("err")),
			),
		),
		jen.Var().Id("items").Op("=").Make(jen.Index().Op("*").Id(m.Name), jen.Id("len").Call(jen.Id("data.Items"))),
		jen.For(jen.List(jen.Id("i"), jen.Id("item"))).Op(":=").Range().Id("data.Items").Block(
			jen.Id("items").Index(jen.Id("i")).Op("=").Op("&").Id(m.Name).Block(
				generateModelResolvers(m, "data.Includes")...,
			),
		),
		jen.Id("it.items").Op("=").Id("items"),
		jen.Return(jen.Id("nil")),
	)

	f.Comment(fmt.Sprintf("%s %s", m.Name, m.Description))
	f.Type().Id(m.Name).Struct(generateModelAttributes(m)...)

	f.Comment(fmt.Sprintf("%sItem contains a single contentful %s model", m.DowncasedName(), m.Name))
	f.Type().Id(fmt.Sprintf("%sItem", m.DowncasedName())).Struct(
		jen.Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
		jen.Id("Fields").Struct(generateModelItemAttributes(m)...).Tag(map[string]string{"json": "fields"}),
	)

	f.Comment(fmt.Sprintf("%sResponse holds an entire contentful %s response", m.DowncasedName(), m.Name))
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
		jen.For(jen.List(jen.Id("_"), jen.Id("entry"))).Op(":=").Range().Id("includes.Entries").Block(
			jen.If(jen.Id("entry.Sys.ID").Op("==").Id("entryID")).Block(
				jen.If(jen.Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Params(jen.Op("*").Id("entry.Fields"), jen.Op("&").Id("item.Fields")).Op(";").Id("err").Op("!=").Id("nil")).Block(
					jen.Return(jen.Id(m.Name).Block()),
				),
				jen.Return(jen.Id(m.Name).Block(generateModelResolvers(m, "includes")...)),
			),
		),
		jen.Return(jen.Id(m.Name).Block()),
	)

	f.Func().Id(fmt.Sprintf("resolve%ss", m.CapitalizedName())).Params(
		jen.Id("ids").Id("entryIDs"),
		jen.Id("includes").Id("includes"),
	).Index().Id(m.Name).Block(
		jen.Var().Id("items").Index().Id(m.Name),
		jen.Var().Id("item").Id(fmt.Sprintf("%sItem", m.DowncasedName())),

		jen.For(jen.List(jen.Id("_"), jen.Id("entry"))).Op(":=").Range().Id("includes.Entries").Block(
			jen.Var().Id("included").Op("=").Lit(false),
			jen.For(jen.List(jen.Id("_"), jen.Id("entryID"))).Op(":=").Range().Id("ids").Block(
				jen.Id("included").Op("=").Id("included").Op("||").Id("entryID.Sys.ID").Op("==").Id("entry.Sys.ID"),
			),
			jen.If(jen.Id("included").Op("==").Lit(true)).Block(
				jen.If(jen.Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Params(jen.Op("*").Id("entry.Fields"), jen.Op("&").Id("item.Fields")).Op(";").Id("err").Op("!=").Id("nil")).Block(
					jen.Return(jen.Id("items")),
				),
				jen.Id("items").Op("=").Append(jen.Id("items"), jen.Id(m.Name).Block(generateModelResolvers(m, "includes")...)),
			),
		),
		jen.Return(jen.Id("items")),
	)

	resolverName := inflector.Pluralize(m.Name)
	f.Comment(fmt.Sprintf("%s retrieves paginated %s entries", resolverName, m.Name))
	f.Func().Params(
		jen.Id("c").Op("*").Id("Client"),
	).Id(resolverName).Params(
		jen.Id("opts").Id("ListOptions"),
	).Call(
		jen.Op("*").Id(fmt.Sprintf("%sIterator", m.Name)),
	).Block(
		jen.If(jen.Id("opts.Limit").Op("<=").Lit(0)).Block(
			jen.Id("opts.Limit").Op("=").Lit(100),
		),
		jen.Id("it").Op(":=").Op("&").Id(fmt.Sprintf("%sIterator", m.Name)).Block(
			jen.Id("Page").Op(":").Id("opts.Page").Op(","),
			jen.Id("Limit").Op(":").Id("opts.Limit").Op(","),
			jen.Id("IncludeCount").Op(":").Id("opts.IncludeCount").Op(","),
			jen.Id("c").Op(":").Id("c").Op(","),
		),
		jen.Return(jen.Id("it")),
	)
}

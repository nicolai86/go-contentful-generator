package main

import "github.com/davelondon/jennifer/jen"

func generateResponseTypes(f *jen.File) {
	f.Type().Id("includes").Struct(
		jen.Id("Entries").Index().Id("includeEntry").Tag(map[string]string{"json": "Entry"}),
		jen.Id("Assets").Index().Id("includeAsset").Tag(map[string]string{"json": "Asset"}),
	)

	f.Type().Id("sys").Struct(
		jen.Id("ID").String().Tag(map[string]string{"json": "id"}),
		jen.Id("Type").String().Tag(map[string]string{"json": "type"}),
		jen.Id("ContentType").Struct(
			jen.Id("Sys").Struct(
				jen.Id("ID").String().Tag(map[string]string{"json": "id"}),
			).Tag(map[string]string{"json": "sys"}),
		).Tag(map[string]string{"json": "contentType"}),
	)

	f.Type().Id("entryID").Struct(
		jen.Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
	)
	f.Type().Id("entryIDs").Index().Id("entryID")

	f.Type().Id("includeEntry").Struct(
		jen.Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
		jen.Id("Fields").Op("*").Qual("encoding/json", "RawMessage").Tag(map[string]string{"json": "fields"}),
	)

	f.Type().Id("includeAsset").Struct(
		jen.Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
		jen.Id("Fields").Struct(
			jen.Id("File").Struct(
				jen.Id("URL").String().Tag(map[string]string{"json": "url"}),
				jen.Id("Details").Struct(
					jen.Id("Image").Struct(
						jen.Id("Width").Int64().Tag(map[string]string{"json": "width"}),
						jen.Id("Height").Int64().Tag(map[string]string{"json": "height"}),
					).Tag(map[string]string{"json": "image"}),
				).Tag(map[string]string{"json": "details"}),
			).Tag(map[string]string{"json": "file"}),
		).Tag(map[string]string{"json": "fields"}),
	)
}

func generateDateType(f *jen.File) {
	f.Const().Id("dateLayout").Op("=").Lit("2006-01-02")
	f.Comment("Date defines an ISO 8601 date only time")
	f.Type().Id("Date").Qual("time", "Time")

	f.Comment("UnmarshalJSON deserializes an iso 8601 short date string")
	f.Func().Params(
		jen.Id("d").Op("*").Id("Date"),
	).Id("UnmarshalJSON").Params(
		jen.Id("b").Index().Byte(),
	).Id("error").Block(
		jen.Id("s").Op(":=").Qual("strings", "Trim").Call(
			jen.Id("string").Parens(jen.Id("b")),
			jen.Lit("\""),
		),
		jen.If(jen.Id("s").Op("==").Lit("null")).Block(
			jen.Op("*").Id("d").Op("=").Id("Date").Call(jen.Qual("time", "Time").Dict(nil)),
		),
		jen.List(jen.Id("t"), jen.Err().Op(":=").Qual("time", "Parse").Call(jen.Id("dateLayout"), jen.Id("s"))),
		jen.Op("*").Id("d").Op("=").Id("Date").Call(jen.Id("t")),
		jen.Return(jen.Err()),
	)
}

func generateAssetType(f *jen.File) {
	f.Comment("Asset defines a media item in contentful")
	f.Type().Id("Asset").Struct(
		jen.Id("Title").String(),
		jen.Id("Description").String(),
		jen.Id("URL").String(),
		jen.Id("Width").Int64(),
		jen.Id("Height").Int64(),
		jen.Id("Size").Int64(),
	)
}

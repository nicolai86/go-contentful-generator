package main

import (
	"fmt"
	"log"
	"os"

	"github.com/davelondon/jennifer/jen"
)

// ContentfulCDNURL is the public domain of contentfuls public CDN
const ContentfulCDNURL = "cdn.contentful.com"

func generateClient(f *jen.File) {
	f.Var().Id("IteratorDone").Id("error").Op("=").Qual("fmt", "Errorf").Params(jen.Lit("IteratorDone"))
	f.Type().Id("ListOptions").Struct(
		jen.Id("Page").Int(),
		jen.Id("Limit").Int(),
		jen.Id("IncludeCount").Int(),
	)

	// TODO return error if not resolvable
	f.Func().Id("resolveAsset").Params(
		jen.Id("assetID").String(),
		jen.Id("includes").Id("includes"),
	).Id("Asset").Block(
		jen.For(
			jen.List(jen.Id("_"), jen.Id("asset"))).Op(":=").Range().Id("includes.Assets").Block(
			jen.If(jen.Id("asset.Sys.ID").Op("==").Id("assetID")).Block(
				jen.Return(jen.Id("Asset").Block(
					jen.Id("URL").Op(":").Qual("fmt", "Sprintf").Params(jen.Lit("https:%s"), jen.Id("asset.Fields.File.URL")).Op(","),
					jen.Id("Width").Op(":").Id("asset.Fields.File.Details.Image.Width").Op(","),
					jen.Id("Height").Op(":").Id("asset.Fields.File.Details.Image.Height").Op(","),
					jen.Id("Size").Op(":").Lit(0).Op(","),
				)),
			),
		),
		jen.Return(jen.Id("Asset").Block()),
	)

	var sts = make([]jen.Code, 0)
	for _, m := range models {
		sts = append(sts,
			jen.If(jen.Id("entry.Sys.ContentType.Sys.ID").Op("==").Lit(m.Sys.ID)).Block(
				jen.Id("items").Op("=").Append(jen.Id("items"), jen.Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Params(jen.Id("entry.Sys.ID"), jen.Id("includes"))),
			),
		)
	}
	f.Func().Id("resolveEntries").Params(
		jen.Id("ids").Id("entryIDs"),
		jen.Id("includes").Id("includes"),
	).Index().Interface().Block(
		jen.Var().Id("items").Index().Interface(),

		jen.For(jen.List(jen.Id("_"), jen.Id("entry"))).Op(":=").Range().Id("includes.Entries").Block(
			jen.Var().Id("included").Op("=").Lit(false),
			jen.For(jen.List(jen.Id("_"), jen.Id("entryID"))).Op(":=").Range().Id("ids").Block(
				jen.Id("included").Op("=").Id("included").Op("||").Id("entryID.Sys.ID").Op("==").Id("entry.Sys.ID"),
			),
			jen.If(jen.Id("included").Op("==").Lit(true)).Block(sts...,
			),
		),
		jen.Return(jen.Id("items")),
	)

	sts = make([]jen.Code, 0)
	for _, m := range models {
		sts = append(sts,
			jen.If(jen.Id("entry.Sys.ContentType.Sys.ID").Op("==").Lit(m.Sys.ID)).Block(
				jen.Return(jen.Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Params(jen.Id("entry.Sys.ID"), jen.Id("includes"))),
			),
		)
	}
	f.Func().Id("resolveEntry").Params(
		jen.Id("id").Id("entryID"),
		jen.Id("includes").Id("includes"),
	).Interface().Block(
		jen.For(jen.List(jen.Id("_"), jen.Id("entry"))).Op(":=").Range().Id("includes.Entries").Block(
			jen.If(jen.Id("entry.Sys.ID").Op("==").Id("id.Sys.ID")).Block(sts...),
		),
		jen.Return(jen.Id("nil")),
	)

	f.Comment("Client")
	f.Type().Id("Client").Struct(
		jen.Id("host").String(),
		jen.Id("spaceID").String(),
		jen.Id("authToken").String(),
		jen.Id("Locales").Index().String(),
		jen.Id("client").Op("*").Qual("net/http", "Client"),
		jen.Id("pool").Op("*").Qual("crypto/x509", "CertPool"),
	)

	f.Const().Id("ContentfulCDNURL").Op("=").Lit(ContentfulCDNURL)
	cert, err := fetchCerts()
	if err != nil {
		log.Fatal(err)
	}

	// TODO include cert pool
	f.Comment("New returns a contentful client")
	f.Func().Id("New").Params(
		jen.Id("authToken").String(),
		jen.Id("locales").Index().String(),
	).Op("*").Id("Client").Block(
		jen.Id("pool").Op(":=").Qual("crypto/x509", "NewCertPool").Params(),
		jen.Sel(jen.Id("pool"), jen.Id("AppendCertsFromPEM")).Params(jen.Id("[]byte").Params(jen.Lit(cert))),
		jen.Return(jen.Op("&").Id("Client").Block(
			jen.Id("host").Op(":").Qual("fmt", "Sprintf").Params(jen.Lit("https://%s"), jen.Id("ContentfulCDNURL")).Op(","),
			jen.Id("spaceID").Op(":").Lit(os.Getenv("CONTENTFUL_SPACE_ID")).Op(","),
			jen.Id("authToken").Op(":").Id("authToken").Op(","),
			jen.Id("Locales").Op(":").Id("locales").Op(","),
			jen.Id("pool").Op(":").Id("pool").Op(","),
			jen.Id("client").Op(":").Op("&").Qual("net/http", "Client").Block(
				jen.Id("Transport").Op(":").Op("&").Qual("net/http", "Transport").Block(
					jen.Id("TLSClientConfig").Op(":").Op("&").Qual("crypto/tls", "Config").Block(
						jen.Id("RootCAs").Op(":").Id("pool").Op(","),
					).Op(","),
				).Op(","),
			).Op(","),
		)),
	)
}

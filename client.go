package main

import (
	"fmt"
	"os"

	"github.com/dave/jennifer/jen"
)

func generateContentClient(f *jen.File) {
	f.Func().Id("resolveAsset").Params(
		jen.Id("assetID").String(),
		jen.Id("includes").Id("includes"),
	).Id("Asset").Block(
		jen.For(
			jen.List(jen.Id("_"), jen.Id("asset")).Op(":=").Range().Id("includes.Assets"),
		).Block(
			jen.If(jen.Id("asset.Sys.ID").Op("==").Id("assetID")).Block(
				jen.Return(jen.Id("Asset").Values(jen.Dict{
					jen.Id("URL"):    jen.Qual("fmt", "Sprintf").Call(jen.Lit("https:%s"), jen.Id("asset.Fields.File.URL")),
					jen.Id("Width"):  jen.Id("asset.Fields.File.Details.Image.Width"),
					jen.Id("Height"): jen.Id("asset.Fields.File.Details.Image.Height"),
					jen.Id("Size"):   jen.Lit(0),
				})),
			),
		),
		jen.Return(jen.Id("Asset").Values()),
	)

	f.Func().Id("resolveEntries").Params(
		jen.Id("ids").Id("entryIDs"),
		jen.Id("its").Index().Id("includeEntry"),
		jen.Id("includes").Id("includes"),
		jen.Id("cache").Id("*iteratorCache"),
	).Index().Interface().Block(
		jen.Var().Id("items").Index().Interface(),

		jen.For(jen.List(jen.Id("_"), jen.Id("entry")).Op(":=").Range().Id("includes.Entries")).Block(
			jen.Var().Id("included").Op("=").Lit(false),
			jen.For(jen.List(jen.Id("_"), jen.Id("entryID")).Op(":=").Range().Id("ids")).Block(
				jen.Id("included").Op("=").Id("included").Op("||").Id("entryID.Sys.ID").Op("==").Id("entry.Sys.ID"),
			),
			jen.If(jen.Id("included").Op("==").Lit(true)).BlockFunc(func(g *jen.Group) {
				for _, m := range models {
					g.If(jen.Id("entry.Sys.ContentType.Sys.ID").Op("==").Lit(m.Sys.ID)).Block(
						jen.Id("items").Op("=").Append(jen.Id("items"), jen.Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Call(
							jen.Id("entry.Sys.ID"),
							jen.Id("its"),
							jen.Id("includes"),
							jen.Id("cache"),
						)),
					)
				}
			}),
		),
		jen.Return(jen.Id("items")),
	)

	f.Func().Id("resolveEntry").Params(
		jen.Id("id").Id("entryID"),
		jen.Id("its").Index().Id("includeEntry"),
		jen.Id("includes").Id("includes"),
		jen.Id("cache").Id("*iteratorCache"),
	).Interface().Block(
		jen.For(jen.List(jen.Id("_"), jen.Id("entry")).Op(":=").Range().Id("includes.Entries")).Block(
			jen.If(jen.Id("entry.Sys.ID").Op("==").Id("id.Sys.ID")).BlockFunc(func(g *jen.Group) {
				for _, m := range models {
					g.If(jen.Id("entry.Sys.ContentType.Sys.ID").Op("==").Lit(m.Sys.ID)).Block(
						jen.Return(jen.Id(fmt.Sprintf("resolve%s", m.CapitalizedName())).Call(
							jen.Id("entry.Sys.ID"),
							jen.Id("its"),
							jen.Id("includes"),
							jen.Id("cache"),
						)),
					)
				}
			}),
		),
		jen.Return(jen.Nil()),
	)

	f.Comment("ContentClient implements a space specific contentful client")
	f.Type().Id("ContentClient").Struct(
		jen.Id("host").String(),
		jen.Id("spaceID").String(),
		jen.Id("authToken").String(),
		jen.Id("Locale").String(),
		jen.Id("environment").String(),
		jen.Id("client").Op("*").Qual("net/http", "Client"),
		jen.Id("pool").Op("*").Qual("crypto/x509", "CertPool"),
	)

	f.Comment("contentfulCDAURL points to the contentful delivery api endpoint")
	f.Const().Id("contentfulCDAURL").Op("=").Lit(cdaEndpoint)

	f.Comment("contentfulCDAURL points to the contentful preview api endpoint")
	f.Const().Id("contentfulCPAURL").Op("=").Lit(cpaEndpoint)

	f.Comment("contentfulCDAURL points to the contentful management api endpoint")
	f.Const().Id("contentfulCMAURL").Op("=").Lit(cmaEndpoint)

	f.Comment("NewCDA returns a contentful client interfacing with the content delivery api")
	f.Func().Id("NewCDA").Params(
		jen.Id("authToken").String(),
		jen.Id("locale").String(),
		jen.Id("environment").String(),
	).Op("*").Id("ContentClient").Block(
		jen.Id("pool").Op(":=").Qual("crypto/x509", "NewCertPool").Call(),
		jen.Id("pool").Dot("AppendCertsFromPEM").Call(jen.Index().Byte().Parens(jen.Lit(certs))),
		jen.Return(jen.Op("&").Id("ContentClient").Values(jen.Dict{
			jen.Id("host"):        jen.Qual("fmt", "Sprintf").Params(jen.Lit("https://%s"), jen.Id("contentfulCDAURL")),
			jen.Id("spaceID"):     jen.Lit(os.Getenv("CONTENTFUL_SPACE_ID")),
			jen.Id("authToken"):   jen.Id("authToken"),
			jen.Id("Locale"):      jen.Id("locale"),
			jen.Id("environment"): jen.Id("environment"),
			jen.Id("pool"):        jen.Id("pool"),
			jen.Id("client"): jen.Op("&").Qual("net/http", "Client").Values(jen.Dict{
				jen.Id("Transport"): jen.Op("&").Qual("net/http", "Transport").Values(jen.Dict{
					jen.Id("TLSClientConfig"): jen.Op("&").Qual("crypto/tls", "Config").Values(jen.Dict{
						jen.Id("RootCAs"): jen.Id("pool"),
					}),
				}),
			}),
		})),
	)

	f.Comment("NewCPA returns a contentful client interfacing with the content preview api")
	f.Func().Id("NewCPA").Params(
		jen.Id("authToken").String(),
		jen.Id("locale").String(),
		jen.Id("environment").String(),
	).Op("*").Id("ContentClient").Block(
		jen.Id("pool").Op(":=").Qual("crypto/x509", "NewCertPool").Call(),
		jen.Id("pool").Dot("AppendCertsFromPEM").Call(jen.Index().Byte().Parens(jen.Lit(certs))),
		jen.Return(jen.Op("&").Id("ContentClient").Values(jen.Dict{
			jen.Id("host"):        jen.Qual("fmt", "Sprintf").Params(jen.Lit("https://%s"), jen.Id("contentfulCPAURL")),
			jen.Id("spaceID"):     jen.Lit(os.Getenv("CONTENTFUL_SPACE_ID")),
			jen.Id("authToken"):   jen.Id("authToken"),
			jen.Id("Locale"):      jen.Id("locale"),
			jen.Id("environment"): jen.Id("environment"),
			jen.Id("pool"):        jen.Id("pool"),
			jen.Id("client"): jen.Op("&").Qual("net/http", "Client").Values(jen.Dict{
				jen.Id("Transport"): jen.Op("&").Qual("net/http", "Transport").Values(jen.Dict{
					jen.Id("TLSClientConfig"): jen.Op("&").Qual("crypto/tls", "Config").Values(jen.Dict{
						jen.Id("RootCAs"): jen.Id("pool"),
					}),
				}),
			}),
		})),
	)
}

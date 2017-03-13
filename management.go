package main

import (
	"fmt"
	"os"

	"github.com/dave/jennifer/jen"
)

func generateManagementClient(f *jen.File) {
	f.Comment("ManagementClient implements a space specific contentful client")
	f.Type().Id("ManagementClient").Struct(
		jen.Id("host").String(),
		jen.Id("spaceID").String(),
		jen.Id("authToken").String(),
		jen.Id("client").Op("*").Qual("net/http", "Client"),
		jen.Id("pool").Op("*").Qual("crypto/x509", "CertPool"),
	)

	f.Comment("NewManagement returns a contentful client interfacing with the content management api")
	f.Func().Id("NewManagement").Params(
		jen.Id("authToken").String(),
	).Op("*").Id("ManagementClient").Block(
		jen.Id("pool").Op(":=").Qual("crypto/x509", "NewCertPool").Call(),
		jen.Id("pool").Dot("AppendCertsFromPEM").Call(jen.Index().Byte().Parens(jen.Lit(certs))),
		jen.Return(jen.Op("&").Id("ManagementClient").Values(jen.Dict{
			jen.Id("host"):      jen.Qual("fmt", "Sprintf").Params(jen.Lit("https://%s"), jen.Id("contentfulCPAURL")),
			jen.Id("spaceID"):   jen.Lit(os.Getenv("CONTENTFUL_SPACE_ID")),
			jen.Id("authToken"): jen.Id("authToken"),
			jen.Id("pool"):      jen.Id("pool"),
			jen.Id("client"): jen.Op("&").Qual("net/http", "Client").Values(jen.Dict{
				jen.Id("Transport"): jen.Op("&").Qual("net/http", "Transport").Values(jen.Dict{
					jen.Id("TLSClientConfig"): jen.Op("&").Qual("crypto/tls", "Config").Values(jen.Dict{
						jen.Id("RootCAs"): jen.Id("pool"),
					}),
				}),
			}),
		})),
	)

	f.Comment("Webhook describes a webhook definition")
	f.Type().Id("Webhook").Struct(
		jen.Id("ID").String().Tag(map[string]string{"json": "-"}),
		jen.Id("Version").Int().Tag(map[string]string{"json": "-"}),
		jen.Id("URL").String().Tag(map[string]string{"json": "url"}),
		jen.Id("Name").String().Tag(map[string]string{"json": "name"}),
		jen.Id("Topics").Index().String().Tag(map[string]string{"json": "topics"}),
	)

	f.Comment("WebhookIterator is used to paginate webhooks")
	f.Type().Id("WebhookIterator").Struct(
		jen.Id("Page").Int(),
		jen.Id("Limit").Int(),
		jen.Id("Offset").Int(),
		jen.Id("c").Op("*").Id("ManagementClient"),
		jen.Id("items").Index().Id("Webhook"),
	)

	f.Type().Id("webhookItem").Struct(
		jen.Id("Sys").Id("sys").Tag(map[string]string{"json": "sys"}),
		jen.Id("Webhook"),
	)

	f.Type().Id("webhooksResponse").Struct(
		jen.Id("Total").Int().Tag(map[string]string{"json": "total"}),
		jen.Id("Skip").Int().Tag(map[string]string{"json": "skip"}),
		jen.Id("Limit").Int().Tag(map[string]string{"json": "limit"}),
		jen.Id("Items").Index().Id("webhookItem").Tag(map[string]string{"json": "items"}),
	)

	f.Comment("Next returns the following item of type Webhook. If none exists a network request will be executed")
	f.Func().Params(
		jen.Id("it").Op("*").Id("WebhookIterator"),
	).Id("Next").Params().Params(
		jen.Op("*").Id("Webhook"), jen.Id("error"),
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
			jen.Return(jen.Nil(), jen.Id("ErrIteratorDone")),
		),
		jen.Var().Id("item").Id("Webhook"),
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
		jen.Return(jen.Id("&item, nil")),
	)

	f.Func().Params(
		jen.Id("it").Op("*").Id("WebhookIterator"),
	).Id("fetch").Params().Id("error").Block(
		jen.Id("c").Op(":=").Id("it.c"),
		jen.Var().Id("url").Op("=").Qual("fmt", "Sprintf").Params(
			jen.Lit(fmt.Sprintf("https://%s/spaces/%%s/webhook_definitions?limit=%%d&skip=%%d", cmaEndpoint)),
			jen.Id("c.spaceID"),
			jen.Id("it.Limit"),
			jen.Id("it.Offset"),
		),
		jen.Var().List(jen.Id("req"), jen.Id("err")).Op("=").Qual("net/http", "NewRequest").Call(
			jen.Lit("GET"),
			jen.Id("url"),
			jen.Nil(),
		),
		jen.If(jen.Err().Op("!=").Nil()).Block(
			jen.Return(jen.Err()),
		),
		jen.Id("req").Dot("Header").Dot("Set").Call(
			jen.Lit("Authorization"),
			jen.Qual("fmt", "Sprintf").Call(jen.Lit("Bearer %s"), jen.Id("c.authToken")),
		),
		jen.Id("req").Dot("Header").Dot("Set").Call(
			jen.Lit("Content-Type"),
			jen.Lit("application/vnd.contentful.management.v1+json"),
		),
		jen.List(jen.Id("resp"), jen.Err()).Op(":=").Id("c.client.Do").Call(jen.Id("req")),
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
		jen.Var().Id("data").Id("webhooksResponse"),
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
		jen.Id("it.items").Op("=").Index().Id("Webhook").Values(),
		jen.For(jen.List(jen.Id("_"), jen.Id("i")).Op(":=").Range().Id("data.Items")).Block(
			jen.Id("i").Dot("Webhook").Dot("ID").Op("=").Id("i").Dot("Sys").Dot("ID"),
			jen.Id("i").Dot("Webhook").Dot("Version").Op("=").Id("i").Dot("Sys").Dot("Version"),
			jen.Id("it").Dot("items").Op("=").Append(jen.Id("it").Dot("items"), jen.Id("i").Dot("Webhook")),
		),
		jen.Return(jen.Nil()),
	)

	f.Comment("List retrieves paginated webhooks")
	f.Func().Params(
		jen.Id("ws").Op("*").Id("WebhookService"),
	).Id("List").Params(
		jen.Id("opts").Id("ListOptions"),
	).Op("*").Id("WebhookIterator").Block(
		jen.If(jen.Id("opts.Limit").Op("<=").Lit(0)).Block(
			jen.Id("opts.Limit").Op("=").Lit(100),
		),

		jen.Id("it").Op(":=").Op("&").Id("WebhookIterator").Values(jen.Dict{
			jen.Id("Page"):  jen.Id("opts.Page"),
			jen.Id("Limit"): jen.Id("opts.Limit"),
			jen.Id("c"):     jen.Id("ws").Dot("client"),
		}),
		jen.Return(jen.Id("it")),
	)

	f.Comment("Create adds a new webhook definitions")
	f.Func().Params(
		jen.Id("ws").Op("*").Id("WebhookService"),
	).Id("Create").Params(jen.Id("w").Op("*").Id("Webhook")).Params(jen.Id("error")).Block(
		jen.Var().Id("url").Op("=").Qual("fmt", "Sprintf").Params(
			jen.Lit(fmt.Sprintf("https://%s/spaces/%%s/webhook_definitions", cmaEndpoint)),
			jen.Id("ws.client.spaceID"),
		),
		jen.Id("b").Op(":=").Qual("bytes", "Buffer").Values(),
		jen.Var().Id("payload").Op("=").Id("webhookItem").Values(
			jen.Dict{
				jen.Id("Webhook"): jen.Op("*").Id("w"),
			},
		),
		jen.If(
			jen.Err().Op(":=").Qual("encoding/json", "NewEncoder").Call(jen.Id("&b")).Dot("Encode").Call(jen.Id("payload")),
			jen.Err().Op("!=").Nil(),
		).Block(jen.Return(jen.Err())),
		jen.Var().List(jen.Id("req"), jen.Id("err")).Op("=").Qual("net/http", "NewRequest").Call(
			jen.Lit("POST"),
			jen.Id("url"),
			jen.Op("&").Id("b"),
		),
		jen.If(jen.Err().Op("!=").Nil()).Block(
			jen.Return(jen.Err()),
		),
		jen.Id("req").Dot("Header").Dot("Set").Call(
			jen.Lit("Authorization"),
			jen.Qual("fmt", "Sprintf").Call(jen.Lit("Bearer %s"), jen.Id("ws.client.authToken")),
		),
		jen.Id("req").Dot("Header").Dot("Set").Call(
			jen.Lit("Content-Type"),
			jen.Lit("application/vnd.contentful.management.v1+json"),
		),
		jen.List(jen.Id("resp"), jen.Err()).Op(":=").Id("ws.client.client.Do").Call(jen.Id("req")),
		jen.If(jen.Err().Op("!=").Nil()).Block(
			jen.Return(jen.Err()),
		),
		jen.If(jen.Id("resp").Dot("StatusCode").Op("!=").Qual("net/http", "StatusCreated")).Block(
			jen.Return(
				jen.Qual("fmt", "Errorf").Call(
					jen.Lit("Request failed: %s, %v"),
					jen.Id("resp.Status"),
					jen.Err(),
				),
			),
		),
		jen.If(
			jen.Err().Op(":=").Qual("encoding/json", "NewDecoder").Call(jen.Id("resp.Body")).Dot("Decode").Call(jen.Id("&payload")),
			jen.Err().Op("!=").Nil(),
		).Block(jen.Return(jen.Err())),
		jen.Id("w.ID").Op("=").Id("payload.Sys.ID"),
		jen.Id("w.Version").Op("=").Id("payload.Sys.Version"),
		jen.Return(jen.Nil()),
	)

	f.Comment("Update changes an existing webhook definitions")
	f.Func().Params(
		jen.Id("ws").Op("*").Id("WebhookService"),
	).Id("Update").Params(jen.Id("w").Op("*").Id("Webhook")).Params(jen.Id("error")).Block(
		jen.Var().Id("url").Op("=").Qual("fmt", "Sprintf").Params(
			jen.Lit(fmt.Sprintf("https://%s/spaces/%%s/webhook_definitions/%%s", cmaEndpoint)),
			jen.Id("ws.client.spaceID"),
			jen.Id("w.ID"),
		),
		jen.Id("b").Op(":=").Qual("bytes", "Buffer").Values(),
		jen.Var().Id("payload").Op("=").Id("webhookItem").Values(
			jen.Dict{
				jen.Id("Webhook"): jen.Op("*").Id("w"),
			},
		),
		jen.If(
			jen.Err().Op(":=").Qual("encoding/json", "NewEncoder").Call(jen.Id("&b")).Dot("Encode").Call(jen.Id("payload")),
			jen.Err().Op("!=").Nil(),
		).Block(jen.Return(jen.Err())),
		jen.Var().List(jen.Id("req"), jen.Id("err")).Op("=").Qual("net/http", "NewRequest").Call(
			jen.Lit("PUT"),
			jen.Id("url"),
			jen.Op("&").Id("b"),
		),
		jen.If(jen.Err().Op("!=").Nil()).Block(
			jen.Return(jen.Err()),
		),
		jen.Id("req").Dot("Header").Dot("Set").Call(
			jen.Lit("Authorization"),
			jen.Qual("fmt", "Sprintf").Call(jen.Lit("Bearer %s"), jen.Id("ws.client.authToken")),
		),
		jen.Id("req").Dot("Header").Dot("Set").Call(
			jen.Lit("X-Contentful-Version"),
			jen.Qual("fmt", "Sprintf").Call(jen.Lit("%d"), jen.Id("w.Version")),
		),
		jen.Id("req").Dot("Header").Dot("Set").Call(
			jen.Lit("Content-Type"),
			jen.Lit("application/vnd.contentful.management.v1+json"),
		),
		jen.List(jen.Id("resp"), jen.Err()).Op(":=").Id("ws.client.client.Do").Call(jen.Id("req")),
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
		jen.If(
			jen.Qual("encoding/json", "NewDecoder").Call(jen.Id("resp.Body")).Dot("Decode").Call(jen.Id("&payload")),
			jen.Err().Op("!=").Nil(),
		).Block(jen.Return(jen.Err())),
		jen.Id("*w").Op("=").Id("payload.Webhook"),
		jen.Return(jen.Nil()),
	)

	f.Comment("Delete adds a new webhook definitions")
	f.Func().Params(
		jen.Id("ws").Op("*").Id("WebhookService"),
	).Id("Delete").Params(jen.Id("id").String()).Params(jen.Id("error")).Block(
		jen.Var().Id("url").Op("=").Qual("fmt", "Sprintf").Params(
			jen.Lit(fmt.Sprintf("https://%s/spaces/%%s/webhook_definitions/%%s", cmaEndpoint)),
			jen.Id("ws.client.spaceID"),
			jen.Id("id"),
		),
		jen.Var().List(jen.Id("req"), jen.Id("err")).Op("=").Qual("net/http", "NewRequest").Call(
			jen.Lit("DELETE"),
			jen.Id("url"),
			jen.Nil(),
		),
		jen.If(jen.Err().Op("!=").Nil()).Block(
			jen.Return(jen.Err()),
		),
		jen.Id("req").Dot("Header").Dot("Set").Call(
			jen.Lit("Authorization"),
			jen.Qual("fmt", "Sprintf").Call(jen.Lit("Bearer %s"), jen.Id("ws.client.authToken")),
		),
		jen.Id("req").Dot("Header").Dot("Set").Call(
			jen.Lit("Content-Type"),
			jen.Lit("application/vnd.contentful.management.v1+json"),
		),
		jen.List(jen.Id("resp"), jen.Err()).Op(":=").Id("ws.client.client.Do").Call(jen.Id("req")),
		jen.If(jen.Err().Op("!=").Nil()).Block(
			jen.Return(jen.Err()),
		),
		jen.If(jen.Id("resp").Dot("StatusCode").Op("!=").Qual("net/http", "StatusNoContent")).Block(
			jen.Return(
				jen.Qual("fmt", "Errorf").Call(
					jen.Lit("Request failed: %s, %v"),
					jen.Id("resp.Status"),
					jen.Err(),
				),
			),
		),
		jen.Return(jen.Nil()),
	)

	f.Comment("WebhookService includes webhook management functions")
	f.Type().Id("WebhookService").Struct(
		jen.Id("client").Op("*").Id("ManagementClient"),
	)

	f.Comment("Webhooks returns a Webhook management service")
	f.Func().Params(
		jen.Id("c").Op("*").Id("ManagementClient"),
	).Id("Webhooks").Params().Params(
		jen.Op("*").Id("WebhookService"),
	).Block(
		jen.Return(
			jen.Op("&").Id("WebhookService").Values(
				jen.Dict{
					jen.Id("client"): jen.Id("c"),
				},
			),
		),
	)
}

// curl -X GET "https://api.contentful.com/spaces/ygx37epqlss8/webhooks/<WEBHOOK_ID>/calls" \
//  -H "Authorization: Bearer " \
//  -H "Content-Type: application/vnd.contentful.management.v1+json"

// curl -X DELETE "https://api.contentful.com/spaces/ygx37epqlss8/webhook_definitions/5VjkUaUpNRrjSKYHvv5zqC" \
//  -H "Authorization: Bearer " \
//  -H "Content-Type: application/vnd.contentful.management.v1+json"

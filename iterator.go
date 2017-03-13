package main

import "github.com/dave/jennifer/jen"

func generateIteratorUtils(f *jen.File) {
	f.Comment("ErrIteratorDone is used to indicate that the iterator has no more data")
	f.Var().Id("ErrIteratorDone").Op("=").Qual("fmt", "Errorf").Call(jen.Lit("IteratorDone"))

	f.Comment("ListOptions contains pagination configuration for iterators")
	f.Type().Id("ListOptions").Struct(
		jen.Id("Page").Int(),
		jen.Id("Limit").Int(),
		jen.Id("IncludeCount").Int(),
	)
}

package main

import "github.com/dave/jennifer/jen"

func merge(dicts ...jen.Dict) jen.Dict {
	d := jen.Dict{}
	for _, dict := range dicts {
		for k, v := range dict {
			d[k] = v
		}
	}
	return d
}

# go-contentful-generator

generate a complete contentful SDK client from your existing schema

## Installation

```
go get -u github.com/nicolai86/go-contentful-generator
```

## Examples

See the test folder for an example usage as well as an example client.

## Usage

first, export the necessary credentials into your env:

```
$ export CONTENTFUL_SPACE_ID=awesome-space
$ export CONTENTFUL_AUTH_TOKEN=secret-token
```

then generate your package: 

```
$ go-contentful-generator -pkg contentful -o contentful.go
```

Or, you can use a go-generate flag like this:

```
//go:generate go-contentful-generator -pkg main -o contentful.go
```

## TODO

- [ ] multi-language schema
- [ ] tests
- [ ] certificate pinning
- [ ] generic entry resolver (return []interface{} )
- [ ] generic entry by id resolver (return interface{} )

# go-contentful-generator

generate a complete contentful SDK client from your existing schema

## Features

- [x] generates typed contentful content delivery api SDK
- [x] generates typed contentful content preview api SDK
- [x] generates typed contentful content management api SDK
- [x] supports recursive type definitions
- [x] supports assets
- [x] supports multiple environments

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
$ export CONTENTFUL_ENVIRONMENT=staging
$ export CONTENTFUL_AUTH_TOKEN=secret-token
```

the `CONTENTFUL_AUTH_TOKEN` has to be for the [CMA](https://www.contentful.com/developers/docs/references/authentication/#).

Then generate your package: 

```
$ go-contentful-generator -pkg contentful -o contentful.go
```

Or, you can use a go-generate flag like this:

```
//go:generate go-contentful-generator -pkg main -o contentful.go
```

## TODO

- [ ] multi-language schema
- [ ] content-type management
- [ ] tests

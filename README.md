# opencli

OpenCLI is a specification language, inspired by OpenAPI, for describing command line tools. This facilitates workflows such as code-generation and documentation to keep CLI implementations in sync with their parsing code and manuals.

> This is a work in progress specification actively developed against a small handful of CLI tools via codegen.

## Why?

Command line tools are ubiquitous. They also expose an interface boundary, they have versioning, breaking changes, deprecations and people often depend on the inputs and outputs. That sounds a _lot_ like an API. But it's harder to treat current CLIs as a specification-defined contract. CLI parsing logic often lives in source code. Documentation is sometimes generated from this source code but it can be brittle and involve either using a library that supports some level of documentation generation, or parsing the code into an AST to generate docs based on assumed structures and syntax.

And in the age of large language model powered agents that often write code and run tools, it's increasingly important to have well documented CLI tools that are up to date and behave in an obvious, expected way.

On top of that, developing CLIs and keeping documentation up to date is becoming part of more and more products. The easiest way to get agents using your product is via CLI so many developers are investing in designing command line tools to pair with their products and services.

So through experience of working with many spec-defined and code-generated workflows, we've noticed that agents really excel at working with these kinds of tools. For example, instead of haphazardly updating HTTP handler logic, parameter parsing, JSON structures, we've notice agents work much more effectively with OpenAPI or similar specifications that generate type-safe code. The spec becomes the source of truth, the contract boundary that defines both the written behavioural semantics as well as the strongly typed inputs and outputs. Paired with code-generation, agents are working in a much more constrained environment where the spec is in charge.

We think CLIs look enough like APIs that they deserve a spec of their own.

## JSON Schema

The OpenCLI JSON Schema is available at:

- `schema/opencli.schema.json`

## Structure

Much like OpenAPI, an OpenCLI spec defines top level schema parts for entrypoints (commands, subcommands), schemas (structured output types), flags and other components of a CLI.

## Code generation

Generators are identified by a `<language>-<framework>` tag so that multiple
frameworks can coexist for the same language. The built-in generators are
`go-cobra` and `rust-clap`.

```sh
opencli generate go-cobra opencli.yaml \
  --package cligen \
  --output ./internal/cligen

opencli generate rust-clap opencli.yaml \
  --module opencli_gen \
  --output ./src
```

Each generator is a separate package with its own help and flags. Specifications
are validated and converted into a shared, language-neutral IR before a
generator runs; component schemas use Schemancer's generic schema IR. Source
layouts are embedded Go templates. Generator packages use underscore-separated
language/framework names such as `go_cobra` and `rust_clap`, while CLI tags use
their hyphenated equivalents.

OpenCLI dogfoods the specification: the `opencli` command is described by
`tools/opencli/opencli.yaml`, and its Cobra command tree is generated into the
`cli` package. Regenerate it with:

```sh
cd tools/opencli
go generate ./cli
```

## Inspiration

- [OpenAPI](https://www.openapis.org/)
- [AsyncAPI](https://www.asyncapi.com/en)
- [docopt](http://docopt.org/)
- [CLIG](https://github.com/cli-guidelines/cli-guidelines)

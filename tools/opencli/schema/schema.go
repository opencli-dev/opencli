// Package schema provides the embedded OpenCLI JSON Schema.
//
// The canonical schema lives at the repository root (schema/opencli.schema.json).
// Because go:embed cannot reference paths outside the module, a copy is kept
// here and refreshed with `go generate ./...`.
package schema

import _ "embed"

//go:generate cp ../../../schema/opencli.schema.json ./opencli.schema.json

// OpenCLI is the OpenCLI JSON Schema (Draft 2020-12), embedded at build time.
//
//go:embed opencli.schema.json
var OpenCLI []byte

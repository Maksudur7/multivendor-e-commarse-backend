package public

import (
	_ "embed"
)

// DocsHTML embeds docs.html directly into the Go binary at build time.
// This guarantees zero-file-system dependencies in Vercel Serverless and cloud environments.
//
//go:embed docs.html
var DocsHTML string

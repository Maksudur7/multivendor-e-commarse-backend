package auth

import "crypto/rand"

// randRead is a package-level alias for crypto/rand.Read.
// Extracted here so handler.go doesn't need a direct import of crypto/rand.
var randRead = rand.Read

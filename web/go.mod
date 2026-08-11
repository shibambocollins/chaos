// This file exists only as a module boundary marker: it walls web/ off
// from the root module's `go build/vet/test ./...` so Go's tooling
// doesn't descend into node_modules looking for stray .go files shipped
// inside npm packages. web/ has no Go code of its own.
module chaos/web-boundary

go 1.25

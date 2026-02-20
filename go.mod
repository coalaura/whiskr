module chat

go 1.26.0

require (
	github.com/coalaura/openingrouter v0.0.0-20260219212935-204abf45d5dd
	github.com/coalaura/plain v1.4.0
	github.com/expr-lang/expr v1.17.8
	github.com/go-chi/chi/v5 v5.2.5
	github.com/goccy/go-yaml v1.19.2
	github.com/revrost/go-openrouter v1.1.6
	github.com/vmihailenco/msgpack/v5 v5.4.1
	golang.org/x/crypto v0.48.0
)

require (
	github.com/coalaura/byteconv v0.1.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.40.0 // indirect
)

replace github.com/revrost/go-openrouter => github.com/coalaura/go-openrouter v0.2.9-0.20260220045232-d278590f7b9c

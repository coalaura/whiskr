package main

import (
	"encoding/json"
	"os"
)

func dump(name string, val any) {
	if !env.Debug {
		return
	}

	b, _ := json.MarshalIndent(val, "", "\t")
	os.WriteFile(name, b, 0644)
}

func debug(format string, args ...any) {
	if !env.Debug {
		return
	}

	log.Printf(format+"\n", args...)
}

func debugIf(cond bool, format string, args ...any) {
	if !cond {
		return
	}

	debug(format, args)
}

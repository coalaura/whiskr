package main

import (
	"encoding/json"
	"os"
)

func dump(v any) {
	if !Debug {
		return
	}

	b, _ := json.MarshalIndent(v, "", "\t")
	os.WriteFile("debug.json", b, 0644)
}

func debug(v any) {
	if !Debug {
		return
	}

	log.Debugf("%#v\n", v)
}

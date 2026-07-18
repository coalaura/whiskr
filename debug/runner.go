package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coalaura/openingrouter"
	"github.com/coalaura/plain"
)

var log = plain.New(plain.WithDate(plain.RFC3339Local))

func main() {
	log.Println("Checking lab logos...")

	err := FindMissingLabLogos()
	log.MustFail(err)

	log.Println("Done")
}

func FindMissingLabLogos() error {
	list, err := openingrouter.ListFrontendModels(context.Background())
	if err != nil {
		return err
	}

	seen := make(map[string]bool, len(list))

	for _, model := range list {
		author := model.Author

		if seen[author] {
			continue
		}

		path := filepath.Join("static", "public", "labs", fmt.Sprintf("%s.png", author))

		_, err = os.Stat(path)
		if os.IsNotExist(err) {
			log.Warnf("[missing-lab] %q\n", author)
		}

		seen[author] = true
	}

	return nil
}

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"

	"github.com/TwiN/deepmerge"
)

const basePath = "pubapi/openapi"

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run pubapi/openapi/generate.go VERSION")
	}

	var overlays = []string{
		fmt.Sprintf("%s.yml", os.Args[1]),
		"ingestion.yml",
	}

	var merged []byte

	for _, overlay := range overlays {
		f, err := os.Open(path.Join(basePath, overlay))
		if err != nil {
			log.Fatal(err)
		}

		yml, err := io.ReadAll(f)
		if err != nil {
			log.Fatal(err)
		}

		merged, err = deepmerge.YAML(merged, yml)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println(string(merged))
}

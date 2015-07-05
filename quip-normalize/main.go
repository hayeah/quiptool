package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hayeah/quiptool"
)

func main() {
	filename := os.Args[1]

	doc, err := quiptool.OpenMarkdown(filename)
	if err != nil {
		log.Fatal(err)
	}

	content, err := doc.NormalizedContent()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(content)
}

package main

import (
	"embed"
	"log"

	"github.com/DiegoOF07/Diskflow/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := desktop.Run(assets); err != nil {
		log.Fatal(err)
	}
}

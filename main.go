package main

import (
	"log"

	"github.com/egoist/mygo"

	"painless/internal/ui/shell"
)

func main() {
	mygo.App.WhenReady(func() {
		shell.Open()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

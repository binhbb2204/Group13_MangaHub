package main

import (
	"fmt"
	"github.com/binhbb2204/Manga-Hub-Group13/cli"
	"os"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"go-url-shortener/cmd/linter/analyze"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(analyze.Analyzer)
}

// Command generate-templates uses templ's generator directly so generation does
// not require the optional development proxy and language-server dependencies.
package main

import (
	"bytes"
	"fmt"
	"github.com/a-h/templ/generator"
	parser "github.com/a-h/templ/parser/v2"
	"go/format"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	file := "web/pages/pages.templ"
	parsed, err := parser.Parse(file)
	if err != nil {
		return err
	}
	var output bytes.Buffer
	if _, err := generator.Generate(parsed, &output, generator.WithFileName(file)); err != nil {
		return err
	}
	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile("web/pages/pages_templ.go", formatted, 0644)
}

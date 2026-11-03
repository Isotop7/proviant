// configdoc generates the configuration reference from the configuration
// itself: the key set comes from the configuration struct, the defaults from
// the default table and the values from the shipped configuration templates.
//
// Usage:
//
//	configdoc --output ../docs/configuration.md
//	configdoc --check
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Isotop7/proviant/models/configuration"
)

// templateNames labels the shipped configuration templates in the reference.
var templateNames = map[string]string{
	"config.yaml.sqlite.tmpl":  "SQLite",
	"config.yaml.mariadb.tmpl": "MariaDB",
}

func main() {
	output := flag.String("output", filepath.Join("..", "docs", "configuration.md"),
		"the file the reference is written to")
	templates := flag.String("templates", "config.yaml.sqlite.tmpl,config.yaml.mariadb.tmpl",
		"the configuration templates the reference reads, comma separated")
	check := flag.Bool("check", false,
		"compare the reference with the generated one instead of writing it")
	flag.Parse()

	if err := run(*output, *templates, *check); err != nil {
		fmt.Fprintln(os.Stderr, "configdoc:", err)
		os.Exit(1)
	}
}

// run generates the reference and either writes it or compares it with the
// file on disk.
func run(output string, templateList string, check bool) error {
	files, err := readTemplates(strings.Split(templateList, ","))
	if err != nil {
		return err
	}
	values, err := configuration.TemplateValues(files)
	if err != nil {
		return err
	}
	generated := configuration.RenderReference(configuration.ConfigKeys(), values)

	if check {
		return compare(output, generated)
	}
	if err := os.WriteFile(output, []byte(generated), 0o600); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", output)
	return nil
}

// readTemplates reads the named configuration templates.
func readTemplates(names []string) ([]configuration.TemplateFile, error) {
	files := make([]configuration.TemplateFile, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		content, err := os.ReadFile(name) //nolint:gosec // G304: the path is the file the operator named
		if err != nil {
			return nil, err
		}
		label, ok := templateNames[filepath.Base(name)]
		if !ok {
			label = filepath.Base(name)
		}
		files = append(files, configuration.TemplateFile{Name: label, Content: content})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no configuration template given")
	}
	return files, nil
}

// compare reports whether the reference on disk matches the generated one.
func compare(output string, generated string) error {
	existing, err := os.ReadFile(output) //nolint:gosec // G304: the path is the file the operator named
	if err != nil {
		return err
	}
	if bytes.Equal(existing, []byte(generated)) {
		return nil
	}
	return fmt.Errorf("%s is out of date — run task configdoc", output)
}

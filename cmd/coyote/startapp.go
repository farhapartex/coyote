package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/farhapartex/coyote/contrib/scaffold"
)

func startApp(args []string) error {
	fs := flag.NewFlagSet("startapp", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	name := fs.Arg(0)
	if name == "" {
		return errors.New("usage: coyote startapp <name>")
	}

	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := checkProject(root); err != nil {
		return err
	}

	feature, err := scaffold.AddFeature(root, name)
	if err != nil {
		return err
	}

	fmt.Printf("created internal/%s\n\n", feature.Package)
	for _, file := range feature.Files {
		fmt.Printf("  %s\n", file)
	}
	fmt.Printf(`
wire it up in main.go:

  import "%s/internal/%s"

  a.RegisterModel(%s.Models()...)
  %s.Routes(a)
  %s.Manage(portal)

then:

  coyote makemigrations --name=add_%s
  coyote migrate
`, moduleOf(root), feature.Package, feature.Package, feature.Package, feature.Package, feature.Package)
	return nil
}

func moduleOf(root string) string {
	body, err := os.ReadFile(root + "/go.mod")
	if err != nil {
		return "your/module"
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if after, found := strings.CutPrefix(line, "module "); found {
			return strings.TrimSpace(after)
		}
	}
	return "your/module"
}

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/farhapartex/coyote/contrib/scaffold"
)

const (
	modulePath = "github.com/farhapartex/coyote"
	toolPath   = modulePath + "/cmd/coyote"
)

func newProject(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	module := fs.String("module", "", "module path for go.mod, defaults to the project name")
	force := fs.Bool("force", false, "scaffold into a directory that is not empty")
	skipDeps := fs.Bool("skip-deps", false, "write the files without running the go toolchain")
	var name string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	if name == "" {
		return errors.New("usage: coyote new <name> [--module=path]")
	}

	project, err := scaffold.Create(scaffold.Options{
		Name:   name,
		Module: *module,
		Force:  *force,
	})
	if err != nil {
		return err
	}

	fmt.Printf("created %s\n\n", relative(project.Dir))
	for _, file := range project.Files {
		fmt.Printf("  %s\n", file)
	}
	fmt.Println()

	if *skipDeps {
		reportManualSteps(project)
		return nil
	}
	if err := resolveDependencies(project); err != nil {
		fmt.Fprintf(os.Stderr, "\ncoyote: %v\n", err)
		reportManualSteps(project)
		return nil
	}

	reportNextSteps(project)
	return nil
}

func resolveDependencies(project scaffold.Project) error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("the go toolchain is not on PATH, so dependencies were not resolved")
	}

	version := frameworkVersion()
	steps := [][]string{
		{"mod", "init", project.Module},
		{"get", modulePath + "@" + version},
		{"get", "-tool", toolPath + "@" + version},
		{"mod", "tidy"},
	}
	for _, step := range steps {
		fmt.Printf("go %s\n", strings.Join(step, " "))
		cmd := exec.Command("go", step...)
		cmd.Dir = project.Dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go %s: %w", strings.Join(step, " "), err)
		}
	}
	return nil
}

func frameworkVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "latest"
	}
	version := info.Main.Version
	if !strings.HasPrefix(version, "v") || strings.Contains(version, "+") {
		return "latest"
	}
	return version
}

func reportNextSteps(project scaffold.Project) {
	fmt.Printf(`
%s is ready.

  cd %s
  coyote start

then, in another terminal, create the schema and an account to sign in with:

  coyote makemigrations --name=initial
  coyote migrate
  coyote createsuperadmin

the site is on http://127.0.0.1:8000/ and the admin portal on http://127.0.0.1:8000/admin/
`, project.Title, relative(project.Dir))
}

func reportManualSteps(project scaffold.Project) {
	version := frameworkVersion()
	fmt.Printf(`
finish the setup yourself with:

  cd %s
  go mod init %s
  go get %s@%s
  go get -tool %s@%s
  go mod tidy
`, relative(project.Dir), project.Module, modulePath, version, toolPath, version)
}

func relative(dir string) string {
	working, err := os.Getwd()
	if err != nil {
		return dir
	}
	rel, err := filepath.Rel(working, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return dir
	}
	return rel
}

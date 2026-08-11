package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/farhapartex/coyote/contrib/cli"
)

func start(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	port := fs.Int("port", 0, "port to listen on")
	host := fs.String("host", "", "host to bind")
	if err := fs.Parse(args); err != nil {
		return err
	}

	env := os.Environ()
	env = append(env, cli.EnvCommand+"="+cli.NameStart)
	if *port != 0 {
		if *port < 1 || *port > 65535 {
			return fmt.Errorf("--port must be between 1 and 65535, got %d", *port)
		}
		env = append(env, cli.EnvPort+"="+strconv.Itoa(*port))
	}
	if *host != "" {
		env = append(env, cli.EnvHost+"="+*host)
	}
	return invoke(env)
}

func migrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	env := append(os.Environ(), cli.EnvCommand+"="+cli.NameMigrate)
	return invoke(env)
}

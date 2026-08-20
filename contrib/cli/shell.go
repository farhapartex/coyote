package cli

import (
	"bufio"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/farhapartex/coyote/core/model"
)

const NameShell = "shell"

type Shell struct{}

func (Shell) Name() string { return NameShell }

func (Shell) Summary() string { return "inspect and query your models interactively" }

func (Shell) Run(ctx Context) error {
	shell, err := newShell(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(ctx.Out, "coyote shell — %d model(s). Type .help for commands, .quit to leave.\n",
		len(ctx.App.Models()))

	reader := bufio.NewReader(ctx.Input())
	for {
		fmt.Fprint(ctx.Out, "> ")
		line, err := reader.ReadString('\n')
		command := strings.TrimSpace(line)

		if command == ".quit" || command == ".exit" {
			return nil
		}
		if command != "" {
			if runErr := shell.run(command); runErr != nil {
				fmt.Fprintf(ctx.Out, "%v\n", runErr)
			}
		}
		if err != nil {
			return nil
		}
	}
}

type shell struct {
	ctx     Context
	records model.Store
	schemas map[string]*model.Schema
	names   []string
}

func newShell(ctx Context) (*shell, error) {
	inspector, ok := ctx.App.(interface {
		Store() (model.Store, error)
		Describe(entity any) (*model.Schema, error)
	})
	if !ok {
		return nil, errors.New("coyote/cli: this application cannot open a shell")
	}
	records, err := inspector.Store()
	if err != nil {
		return nil, err
	}

	out := &shell{ctx: ctx, records: records, schemas: map[string]*model.Schema{}}
	for _, m := range ctx.App.Models() {
		schema, err := inspector.Describe(m.Entity)
		if err != nil {
			continue
		}
		out.schemas[schema.Table] = schema
		out.names = append(out.names, schema.Table)
	}
	sort.Strings(out.names)
	return out, nil
}

func (s *shell) run(line string) error {
	fields := strings.Fields(line)
	switch fields[0] {
	case ".help":
		return s.help()
	case ".models", ".tables":
		fmt.Fprintf(s.ctx.Out, "%s\n", strings.Join(s.names, "\n"))
		return nil
	case ".describe":
		return s.describe(fields)
	case "count":
		return s.count(fields)
	case "list":
		return s.list(fields)
	case "get":
		return s.get(fields)
	}
	return fmt.Errorf("unknown command %q; type .help", fields[0])
}

func (s *shell) help() error {
	fmt.Fprint(s.ctx.Out, `.models             list every registered model
.describe MODEL     show a model's columns
count MODEL         how many rows
list MODEL [N]      the first N rows, default 10
get MODEL ID        one row by primary key
.quit               leave
`)
	return nil
}

func (s *shell) schema(name string) (*model.Schema, error) {
	if schema, ok := s.schemas[name]; ok {
		return schema, nil
	}
	return nil, fmt.Errorf("no model called %q; type .models", name)
}

func (s *shell) describe(fields []string) error {
	if len(fields) < 2 {
		return errors.New("usage: .describe MODEL")
	}
	schema, err := s.schema(fields[1])
	if err != nil {
		return err
	}
	for _, field := range schema.Fields {
		flags := []string{string(field.Kind)}
		if field.PrimaryKey {
			flags = append(flags, "primary key")
		}
		if field.Required {
			flags = append(flags, "required")
		}
		fmt.Fprintf(s.ctx.Out, "  %-24s %s\n", field.Column, strings.Join(flags, ", "))
	}
	return nil
}

func (s *shell) count(fields []string) error {
	if len(fields) < 2 {
		return errors.New("usage: count MODEL")
	}
	schema, err := s.schema(fields[1])
	if err != nil {
		return err
	}
	total, err := s.records.Count(s.ctx.Context(), schema, model.Query{})
	if err != nil {
		return err
	}
	fmt.Fprintf(s.ctx.Out, "%d\n", total)
	return nil
}

func (s *shell) list(fields []string) error {
	if len(fields) < 2 {
		return errors.New("usage: list MODEL [N]")
	}
	schema, err := s.schema(fields[1])
	if err != nil {
		return err
	}
	limit := 10
	if len(fields) > 2 {
		if parsed, err := strconv.Atoi(fields[2]); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	page, err := s.records.List(s.ctx.Context(), schema, model.Query{Limit: limit})
	if err != nil {
		return err
	}
	for _, record := range page.Records {
		s.print(schema, record)
	}
	fmt.Fprintf(s.ctx.Out, "%d of %d row(s)\n", len(page.Records), page.Total)
	return nil
}

func (s *shell) get(fields []string) error {
	if len(fields) < 3 {
		return errors.New("usage: get MODEL ID")
	}
	schema, err := s.schema(fields[1])
	if err != nil {
		return err
	}
	record, err := s.records.Find(s.ctx.Context(), schema, fields[2])
	if err != nil {
		return err
	}
	s.print(schema, record)
	return nil
}

func (s *shell) print(schema *model.Schema, record model.Record) {
	parts := make([]string, 0, len(schema.Fields))
	for _, field := range schema.Visible() {
		parts = append(parts, fmt.Sprintf("%s=%v", field.Column, record.Get(field.Column)))
	}
	fmt.Fprintf(s.ctx.Out, "  %s\n", strings.Join(parts, "  "))
}

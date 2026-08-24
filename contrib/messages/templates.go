package messages

import (
	"strconv"
	"strings"
	"text/template/parse"
)

func FromTemplate(path string, source string) (*Set, []Problem, error) {
	set := NewSet()
	problems := []Problem{}

	tree := parse.New(path)
	tree.Mode = parse.SkipFuncCheck
	trees := map[string]*parse.Tree{}

	if _, err := tree.Parse(source, "", "", trees); err != nil {
		return set, problems, err
	}

	for _, parsed := range trees {
		if parsed.Root == nil {
			continue
		}
		walkNode(parsed.Root, path, source, set, &problems)
	}
	return set, problems, nil
}

func walkNode(node parse.Node, path, source string, set *Set, problems *[]Problem) {
	switch typed := node.(type) {
	case *parse.ListNode:
		if typed == nil {
			return
		}
		for _, child := range typed.Nodes {
			walkNode(child, path, source, set, problems)
		}
	case *parse.ActionNode:
		walkPipe(typed.Pipe, path, source, set, problems)
	case *parse.IfNode:
		walkBranch(typed.Pipe, typed.List, typed.ElseList, path, source, set, problems)
	case *parse.RangeNode:
		walkBranch(typed.Pipe, typed.List, typed.ElseList, path, source, set, problems)
	case *parse.WithNode:
		walkBranch(typed.Pipe, typed.List, typed.ElseList, path, source, set, problems)
	case *parse.TemplateNode:
		walkPipe(typed.Pipe, path, source, set, problems)
	}
}

func walkBranch(pipe *parse.PipeNode, list, elseList *parse.ListNode, path, source string, set *Set, problems *[]Problem) {
	walkPipe(pipe, path, source, set, problems)
	walkNode(list, path, source, set, problems)
	walkNode(elseList, path, source, set, problems)
}

func walkPipe(pipe *parse.PipeNode, path, source string, set *Set, problems *[]Problem) {
	if pipe == nil {
		return
	}
	for _, command := range pipe.Cmds {
		walkCommand(command, path, source, set, problems)
	}
}

func walkCommand(command *parse.CommandNode, path, source string, set *Set, problems *[]Problem) {
	if command == nil || len(command.Args) == 0 {
		return
	}

	for _, argument := range command.Args {
		if nested, ok := argument.(*parse.PipeNode); ok {
			walkPipe(nested, path, source, set, problems)
		}
	}

	name, ok := methodName(command.Args[0])
	if !ok {
		return
	}
	form, known := shapeOf(name)
	if !known {
		return
	}

	line := lineOf(source, command.Position())
	literals := []string{}
	suspect := false
	for _, argument := range command.Args[1:] {
		if text, ok := argument.(*parse.StringNode); ok {
			literals = append(literals, text.Text)
			continue
		}
		switch argument.(type) {
		case *parse.FieldNode, *parse.VariableNode, *parse.ChainNode, *parse.PipeNode:
			suspect = true
		}
	}

	found, built := form.build(literals)
	if !built {
		if suspect {
			*problems = append(*problems, Problem{
				File:   path,
				Line:   line,
				Reason: name + " was called with a value that is not a literal, so it cannot be extracted",
			})
		}
		return
	}
	found.References = []string{path + ":" + strconv.Itoa(line)}
	set.Add(found)
}

func methodName(argument parse.Node) (string, bool) {
	switch typed := argument.(type) {
	case *parse.FieldNode:
		return lastIdent(typed.Ident)
	case *parse.VariableNode:
		return lastIdent(typed.Ident)
	case *parse.ChainNode:
		return lastIdent(typed.Field)
	case *parse.IdentifierNode:
		return typed.Ident, true
	}
	return "", false
}

func lastIdent(chain []string) (string, bool) {
	if len(chain) == 0 {
		return "", false
	}
	return chain[len(chain)-1], true
}

func lineOf(source string, position parse.Pos) int {
	at := int(position)
	if at > len(source) {
		at = len(source)
	}
	return strings.Count(source[:at], "\n") + 1
}

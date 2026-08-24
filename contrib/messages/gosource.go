package messages

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

func FromGo(path string, source []byte) (*Set, []Problem, error) {
	set := NewSet()
	problems := []Problem{}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, source, parser.SkipObjectResolution)
	if err != nil {
		return set, problems, err
	}

	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		form, known := shapeOf(selector.Sel.Name)
		if !known {
			return true
		}

		line := fileSet.Position(call.Pos()).Line
		literals, allConstant := stringArguments(call.Args)
		found, ok := form.build(literals)
		if !ok {
			if !allConstant {
				problems = append(problems, Problem{
					File:   path,
					Line:   line,
					Reason: selector.Sel.Name + " was called with a value that is not a literal, so it cannot be extracted",
				})
			}
			return true
		}
		found.References = []string{path + ":" + strconv.Itoa(line)}
		set.Add(found)
		return true
	})
	return set, problems, nil
}

func stringArguments(args []ast.Expr) ([]string, bool) {
	out := []string{}
	allConstant := true

	for _, argument := range args {
		text, ok := literalOf(argument)
		if !ok {
			if isSuspect(argument) {
				allConstant = false
			}
			continue
		}
		out = append(out, text)
	}
	return out, allConstant
}

func literalOf(expression ast.Expr) (string, bool) {
	switch typed := expression.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(typed.Value)
		if err != nil {
			return "", false
		}
		return text, true
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return "", false
		}
		left, leftOK := literalOf(typed.X)
		right, rightOK := literalOf(typed.Y)
		if !leftOK || !rightOK {
			return "", false
		}
		return left + right, true
	}
	return "", false
}

func isSuspect(expression ast.Expr) bool {
	switch expression.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr, *ast.BinaryExpr:
		return true
	}
	return false
}

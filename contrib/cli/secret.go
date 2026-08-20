package cli

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func (c Context) Interactive() bool {
	file, ok := c.Input().(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func (c Context) ReadSecret(label string) (string, bool, error) {
	if !c.Interactive() {
		return "", false, nil
	}
	file := c.Input().(*os.File)

	fmt.Fprintf(c.Out, "%s: ", label)
	first, err := term.ReadPassword(int(file.Fd()))
	fmt.Fprintln(c.Out)
	if err != nil {
		return "", true, fmt.Errorf("coyote/cli: reading %s: %w", strings.ToLower(label), err)
	}

	fmt.Fprintf(c.Out, "%s again: ", label)
	second, err := term.ReadPassword(int(file.Fd()))
	fmt.Fprintln(c.Out)
	if err != nil {
		return "", true, fmt.Errorf("coyote/cli: reading %s: %w", strings.ToLower(label), err)
	}

	if string(first) != string(second) {
		return "", true, fmt.Errorf("coyote/cli: the two %ss did not match", strings.ToLower(label))
	}
	return strings.TrimSpace(string(first)), true, nil
}

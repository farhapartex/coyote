package cli

import (
	"fmt"
	"sort"
)

func routedElsewhere(ctx Context) []string {
	seen := map[string]bool{}
	for _, m := range ctx.App.Models() {
		if m.Alias != "" && !seen[m.Alias] {
			seen[m.Alias] = true
		}
	}
	out := make([]string, 0, len(seen))
	for alias := range seen {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

func reportRouting(ctx Context) {
	aliases := routedElsewhere(ctx)
	if len(aliases) == 0 {
		return
	}
	fmt.Fprintf(ctx.Out, "\nnot migrated: %d model(s) are routed to another database (%v).\n", countRouted(ctx), aliases)
	fmt.Fprintln(ctx.Out, "migrations run against the default connection only; create those tables yourself for now.")
}

func countRouted(ctx Context) int {
	total := 0
	for _, m := range ctx.App.Models() {
		if m.Alias != "" {
			total++
		}
	}
	return total
}

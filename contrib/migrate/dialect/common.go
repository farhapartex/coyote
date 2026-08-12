package dialect

import (
	"fmt"
	"strings"
)

func joinColumns(d Dialect, columns []string) string {
	quoted := make([]string, 0, len(columns))
	for _, c := range columns {
		quoted = append(quoted, d.Quote(c))
	}
	return strings.Join(quoted, ", ")
}

func columnClause(d Dialect, c Column) string {
	clause := d.Quote(c.Name) + " " + c.Type
	if c.NotNull && !c.PrimaryKey {
		clause += " NOT NULL"
	}
	if c.Default != "" {
		clause += " DEFAULT " + c.Default
	}
	return clause
}

func createTable(d Dialect, table string, columns []Column, primaryKeyInline bool) string {
	lines := make([]string, 0, len(columns)+1)
	keys := make([]string, 0, 1)
	for _, c := range columns {
		clause := columnClause(d, c)
		if c.PrimaryKey {
			if primaryKeyInline {
				clause += " PRIMARY KEY"
			} else {
				keys = append(keys, c.Name)
			}
		}
		lines = append(lines, "  "+clause)
	}
	if len(keys) > 0 {
		lines = append(lines, "  PRIMARY KEY ("+joinColumns(d, keys)+")")
	}
	return fmt.Sprintf("CREATE TABLE %s (\n%s\n)", d.Quote(table), strings.Join(lines, ",\n"))
}

func createIndex(d Dialect, index Index) string {
	unique := ""
	if index.Unique {
		unique = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
		unique, d.Quote(index.Name), d.Quote(index.Table), joinColumns(d, index.Columns))
}

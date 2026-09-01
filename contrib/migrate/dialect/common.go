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
	return columnClauseWith(d, c, c.PrimaryKey)
}

func columnClauseWith(d Dialect, c Column, ownsKey bool) string {
	clause := d.Quote(c.Name) + " " + c.Type
	if c.NotNull && !ownsKey {
		clause += " NOT NULL"
	}
	if c.Default != "" {
		clause += " DEFAULT " + c.Default
	}
	return clause
}

func ForeignKeyName(table string, column string) string {
	return "fk_" + table + "_" + column
}

func foreignKeyClause(d Dialect, table string, c Column) string {
	return fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
		d.Quote(ForeignKeyName(table, c.Name)), d.Quote(c.Name),
		d.Quote(c.References.Table), d.Quote(c.References.Column))
}

func addColumnWithConstraint(d Dialect, table string, c Column) []string {
	out := []string{fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", d.Quote(table), columnClause(d, c))}
	if c.References.IsZero() {
		return out
	}
	return append(out, fmt.Sprintf("ALTER TABLE %s ADD %s", d.Quote(table), foreignKeyClause(d, table, c)))
}

func createTable(d Dialect, table string, columns []Column, primaryKeyInline bool) string {
	return createTableNamed(d, table, table, columns, primaryKeyInline)
}

func createTableNamed(d Dialect, table, constraintTable string, columns []Column, primaryKeyInline bool) string {
	declared := 0
	for _, c := range columns {
		if c.PrimaryKey {
			declared++
		}
	}
	inline := primaryKeyInline && declared == 1

	lines := make([]string, 0, len(columns)+1)
	keys := make([]string, 0, 1)
	for _, c := range columns {
		clause := columnClauseWith(d, c, c.PrimaryKey && inline)
		if c.PrimaryKey {
			if inline {
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
	for _, c := range columns {
		if !c.References.IsZero() {
			lines = append(lines, "  "+foreignKeyClause(d, constraintTable, c))
		}
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

package main

import (
	"fmt"
	"strconv"
	"strings"
)

func money(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	whole := cents / 100
	part := cents % 100

	digits := strconv.FormatInt(whole, 10)
	grouped := groupThousands(digits)

	out := fmt.Sprintf("£%s.%02d", grouped, part)
	if negative {
		return "-" + out
	}
	return out
}

func groupThousands(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	head := len(digits) % 3
	parts := []string{}
	if head > 0 {
		parts = append(parts, digits[:head])
	}
	for i := head; i < len(digits); i += 3 {
		parts = append(parts, digits[i:i+3])
	}
	return strings.Join(parts, ",")
}

func parseMoney(raw string) (int64, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(strings.TrimPrefix(raw, "£"), ",", ""))
	if raw == "" {
		return 0, nil
	}
	whole, fraction, found := strings.Cut(raw, ".")
	pounds, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	if !found {
		return pounds * 100, nil
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	pence, err := strconv.ParseInt(fraction[:2], 10, 64)
	if err != nil {
		return 0, err
	}
	return pounds*100 + pence, nil
}

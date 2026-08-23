package i18n

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type formatter struct {
	locale   *Locale
	patterns patterns
}

func newFormatter(locale *Locale) *formatter {
	return &formatter{locale: locale, patterns: patternsFor(locale.Tag())}
}

func (f *formatter) Date(t time.Time) string {
	return f.locale.inZone(t).Format(f.patterns.date)
}

func (f *formatter) Time(t time.Time) string {
	return f.locale.inZone(t).Format(f.patterns.time)
}

func (f *formatter) DateTime(t time.Time) string {
	return f.locale.inZone(t).Format(f.patterns.dateTime)
}

func (f *formatter) LongDate(t time.Time) string {
	when := f.locale.inZone(t)
	month := f.locale.T(monthNames[int(when.Month())-1])

	replacer := strings.NewReplacer(
		"{day}", strconv.Itoa(when.Day()),
		"{month}", month,
		"{year}", strconv.Itoa(when.Year()),
	)
	return replacer.Replace(f.patterns.longDate)
}

func (f *formatter) Number(value any) string {
	text, negative, ok := decimalOf(value, -1)
	if !ok {
		return fmt.Sprint(value)
	}
	return f.assemble(text, negative)
}

func (f *formatter) Money(value any, currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	text, negative, ok := decimalOf(value, digitsFor(currency))
	if !ok {
		return fmt.Sprint(value)
	}

	amount := f.assemble(text, negative)
	symbol := symbolFor(currency)
	gap := ""
	if f.patterns.moneyGap {
		gap = nbsp
	}
	if f.patterns.moneyLead {
		return symbol + gap + amount
	}
	return amount + gap + symbol
}

func (f *formatter) assemble(text string, negative bool) string {
	whole, fraction, _ := strings.Cut(text, ".")
	out := f.group(whole)
	if fraction != "" {
		out += f.patterns.decimal + fraction
	}
	if negative {
		return "-" + out
	}
	return out
}

func (f *formatter) group(digits string) string {
	separator := f.patterns.group
	if separator == "" || len(digits) <= f.patterns.grouping[0] {
		return digits
	}

	sizes := f.patterns.grouping
	parts := []string{}
	at := len(digits)
	step := 0

	for at > 0 {
		size := sizes[min(step, len(sizes)-1)]
		start := max(at-size, 0)
		parts = append(parts, digits[start:at])
		at = start
		step++
	}

	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return strings.Join(parts, separator)
}

func decimalOf(value any, digits int) (string, bool, bool) {
	switch typed := value.(type) {
	case int:
		return integerText(int64(typed), digits)
	case int8:
		return integerText(int64(typed), digits)
	case int16:
		return integerText(int64(typed), digits)
	case int32:
		return integerText(int64(typed), digits)
	case int64:
		return integerText(typed, digits)
	case uint:
		return integerText(int64(typed), digits)
	case uint8:
		return integerText(int64(typed), digits)
	case uint16:
		return integerText(int64(typed), digits)
	case uint32:
		return integerText(int64(typed), digits)
	case uint64:
		return integerText(int64(typed), digits)
	case float32:
		return floatText(float64(typed), digits)
	case float64:
		return floatText(typed, digits)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return "", false, false
		}
		return floatText(parsed, digits)
	}
	return "", false, false
}

func integerText(value int64, digits int) (string, bool, bool) {
	negative := value < 0
	if negative {
		value = -value
	}
	text := strconv.FormatInt(value, 10)
	if digits > 0 {
		text += "." + strings.Repeat("0", digits)
	}
	return text, negative, true
}

func floatText(value float64, digits int) (string, bool, bool) {
	negative := value < 0
	if negative {
		value = -value
	}
	if digits < 0 {
		return strconv.FormatFloat(value, 'f', -1, 64), negative, true
	}
	return strconv.FormatFloat(value, 'f', digits, 64), negative, true
}

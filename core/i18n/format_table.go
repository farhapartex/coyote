package i18n

const nbsp = "\u00a0"

type patterns struct {
	date      string
	time      string
	dateTime  string
	decimal   string
	group     string
	grouping  []int
	longDate  string
	moneyLead bool
	moneyGap  bool
}

var defaultPatterns = patterns{
	date:      "02/01/2006",
	time:      "15:04",
	dateTime:  "02/01/2006 15:04",
	decimal:   ".",
	group:     ",",
	grouping:  []int{3},
	longDate:  "{day} {month} {year}",
	moneyLead: true,
}

var localePatterns = map[string]patterns{
	"en":    defaultPatterns,
	"en-US": {date: "01/02/2006", time: "3:04 PM", dateTime: "01/02/2006 3:04 PM", decimal: ".", group: ",", grouping: []int{3}, longDate: "{month} {day}, {year}", moneyLead: true},
	"fr":    {date: "02/01/2006", time: "15:04", dateTime: "02/01/2006 15:04", decimal: ",", group: nbsp, grouping: []int{3}, longDate: "{day} {month} {year}", moneyGap: true},
	"de":    {date: "02.01.2006", time: "15:04", dateTime: "02.01.2006 15:04", decimal: ",", group: ".", grouping: []int{3}, longDate: "{day}. {month} {year}", moneyGap: true},
	"es":    {date: "02/01/2006", time: "15:04", dateTime: "02/01/2006 15:04", decimal: ",", group: ".", grouping: []int{3}, longDate: "{day} de {month} de {year}", moneyGap: true},
	"it":    {date: "02/01/2006", time: "15:04", dateTime: "02/01/2006 15:04", decimal: ",", group: ".", grouping: []int{3}, longDate: "{day} {month} {year}", moneyLead: true, moneyGap: true},
	"nl":    {date: "02-01-2006", time: "15:04", dateTime: "02-01-2006 15:04", decimal: ",", group: ".", grouping: []int{3}, longDate: "{day} {month} {year}", moneyLead: true, moneyGap: true},
	"pt":    {date: "02/01/2006", time: "15:04", dateTime: "02/01/2006 15:04", decimal: ",", group: ".", grouping: []int{3}, longDate: "{day} de {month} de {year}", moneyLead: true, moneyGap: true},
	"pt-BR": {date: "02/01/2006", time: "15:04", dateTime: "02/01/2006 15:04", decimal: ",", group: ".", grouping: []int{3}, longDate: "{day} de {month} de {year}", moneyLead: true, moneyGap: true},
	"ru":    {date: "02.01.2006", time: "15:04", dateTime: "02.01.2006 15:04", decimal: ",", group: nbsp, grouping: []int{3}, longDate: "{day} {month} {year}", moneyGap: true},
	"uk":    {date: "02.01.2006", time: "15:04", dateTime: "02.01.2006 15:04", decimal: ",", group: nbsp, grouping: []int{3}, longDate: "{day} {month} {year}", moneyGap: true},
	"pl":    {date: "02.01.2006", time: "15:04", dateTime: "02.01.2006 15:04", decimal: ",", group: nbsp, grouping: []int{3}, longDate: "{day} {month} {year}", moneyGap: true},
	"tr":    {date: "02.01.2006", time: "15:04", dateTime: "02.01.2006 15:04", decimal: ",", group: ".", grouping: []int{3}, longDate: "{day} {month} {year}", moneyGap: true},
	"ar":    {date: "02/01/2006", time: "15:04", dateTime: "02/01/2006 15:04", decimal: ".", group: ",", grouping: []int{3}, longDate: "{day} {month} {year}", moneyLead: true, moneyGap: true},
	"he":    {date: "02.01.2006", time: "15:04", dateTime: "02.01.2006 15:04", decimal: ".", group: ",", grouping: []int{3}, longDate: "{day} {month} {year}", moneyLead: true, moneyGap: true},
	"fa":    {date: "2006/01/02", time: "15:04", dateTime: "2006/01/02 15:04", decimal: ".", group: ",", grouping: []int{3}, longDate: "{day} {month} {year}", moneyLead: true, moneyGap: true},
	"hi":    {date: "02/01/2006", time: "3:04 PM", dateTime: "02/01/2006 3:04 PM", decimal: ".", group: ",", grouping: []int{3, 2}, longDate: "{day} {month} {year}", moneyLead: true},
	"bn":    {date: "02/01/2006", time: "3:04 PM", dateTime: "02/01/2006 3:04 PM", decimal: ".", group: ",", grouping: []int{3, 2}, longDate: "{day} {month} {year}", moneyLead: true},
	"ja":    {date: "2006/01/02", time: "15:04", dateTime: "2006/01/02 15:04", decimal: ".", group: ",", grouping: []int{3}, longDate: "{year}年{month}{day}日", moneyLead: true},
	"zh":    {date: "2006/01/02", time: "15:04", dateTime: "2006/01/02 15:04", decimal: ".", group: ",", grouping: []int{3}, longDate: "{year}年{month}{day}日", moneyLead: true},
	"ko":    {date: "2006. 01. 02.", time: "15:04", dateTime: "2006. 01. 02. 15:04", decimal: ".", group: ",", grouping: []int{3}, longDate: "{year}년 {month} {day}일", moneyLead: true},
	"id":    {date: "02/01/2006", time: "15:04", dateTime: "02/01/2006 15:04", decimal: ",", group: ".", grouping: []int{3}, longDate: "{day} {month} {year}", moneyLead: true, moneyGap: true},
	"vi":    {date: "02/01/2006", time: "15:04", dateTime: "02/01/2006 15:04", decimal: ",", group: ".", grouping: []int{3}, longDate: "{day} {month} {year}", moneyGap: true},
}

var currencySymbols = map[string]string{
	"USD": "$", "EUR": "€", "GBP": "£", "JPY": "¥", "CNY": "¥",
	"INR": "₹", "BDT": "৳", "KRW": "₩", "RUB": "₽", "TRY": "₺",
	"BRL": "R$", "CAD": "CA$", "AUD": "A$", "CHF": "CHF", "SEK": "kr", "PLN": "zł",
	"ILS": "₪", "SAR": "ر.س", "AED": "د.إ", "NGN": "₦",
	"ZAR": "R", "MXN": "MX$", "IDR": "Rp", "VND": "₫", "THB": "฿",
}

var currencyDigits = map[string]int{
	"JPY": 0, "KRW": 0, "VND": 0, "CLP": 0, "ISK": 0, "BIF": 0, "XAF": 0, "XOF": 0,
	"BHD": 3, "KWD": 3, "OMR": 3, "TND": 3, "JOD": 3,
}

var monthNames = []string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

func patternsFor(tag string) patterns {
	if found, ok := localePatterns[Normalise(tag)]; ok {
		return found
	}
	if found, ok := localePatterns[BaseOf(tag)]; ok {
		return found
	}
	return defaultPatterns
}

func symbolFor(currency string) string {
	if symbol, found := currencySymbols[currency]; found {
		return symbol
	}
	return currency
}

func digitsFor(currency string) int {
	if digits, found := currencyDigits[currency]; found {
		return digits
	}
	return 2
}

package mail

import (
	"fmt"
	netmail "net/mail"
	"strings"
)

const maxAddresses = 256

func ParseAddress(value string) (netmail.Address, error) {
	if containsBreak(value) {
		return netmail.Address{}, fmt.Errorf("%w: %q", ErrHeaderInjection, value)
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return netmail.Address{}, fmt.Errorf("%w: empty", ErrBadAddress)
	}

	parsed, err := netmail.ParseAddress(trimmed)
	if err != nil {
		return netmail.Address{}, fmt.Errorf("%w: %q: %w", ErrBadAddress, trimmed, err)
	}
	return *parsed, nil
}

func ParseAddressList(values []string) ([]netmail.Address, error) {
	if len(values) > maxAddresses {
		return nil, fmt.Errorf("%w: %d addresses is more than the %d allowed",
			ErrBadAddress, len(values), maxAddresses)
	}

	out := make([]netmail.Address, 0, len(values))
	for _, value := range values {
		parsed, err := ParseAddress(value)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

func FormatAddress(name, address string) string {
	formatted := netmail.Address{Name: name, Address: address}
	return formatted.String()
}

func FormatAddressList(addresses []netmail.Address) string {
	parts := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address.Name == "" {
			parts = append(parts, address.Address)
			continue
		}
		formatted := address
		parts = append(parts, formatted.String())
	}
	return strings.Join(parts, ", ")
}

func containsBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

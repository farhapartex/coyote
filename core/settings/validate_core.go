package settings

import (
	"strconv"
	"strings"
)

func (s Settings) validateCore(add func(string)) {
	if _, known := ParseProfile(string(s.Environment)); !known {
		add("Environment " + strconv.Quote(string(s.Environment)) +
			" is not recognised; use \"development\", \"staging\" or \"production\"")
	}
	if s.Environment.Deployed() && s.Debug {
		add("Debug must be off when Environment is " + string(s.Environment))
	}

	if s.SecretKey == "" {
		add("SecretKey is empty; set a random value of at least 32 characters (settings.GenerateSecretKey can make one)")
	} else if len(s.SecretKey) < minSecretKeyLength && !s.Debug {
		add("SecretKey is shorter than 32 characters")
	}

	if len(s.AllowedHosts) == 0 && !s.Debug {
		add("AllowedHosts is empty; with Debug disabled you must list the hosts this site serves, or use []string{\"*\"} to allow any")
	}
	for _, host := range s.AllowedHosts {
		if strings.TrimSpace(host) == "" {
			add("AllowedHosts contains an empty entry")
		}
	}

	if s.BaseDir == "" {
		add("BaseDir is empty")
	}
}

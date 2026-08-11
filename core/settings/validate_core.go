package settings

import "strings"

func (s Settings) validateCore(add func(string)) {
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

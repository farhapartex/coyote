package settings

import (
	"strings"
	"time"
)

type Profile string

const (
	Development Profile = "development"
	Staging     Profile = "staging"
	Production  Profile = "production"
)

var profileAliases = map[string]Profile{
	"development": Development,
	"dev":         Development,
	"local":       Development,
	"staging":     Staging,
	"stage":       Staging,
	"production":  Production,
	"prod":        Production,
	"live":        Production,
}

func ParseProfile(name string) (Profile, bool) {
	resolved, ok := profileAliases[strings.ToLower(strings.TrimSpace(name))]
	return resolved, ok
}

func Preset(name string) func(*Settings) {
	return func(s *Settings) {
		resolved, ok := ParseProfile(name)
		if !ok {
			s.Environment = Profile(strings.TrimSpace(name))
			return
		}
		resolved.Apply(s)
	}
}

func (p Profile) Apply(s *Settings) {
	s.Environment = p
	switch p {
	case Development:
		applyDevelopment(s)
	case Staging, Production:
		applyDeployed(s)
	}
}

func (p Profile) Deployed() bool { return p == Staging || p == Production }

func applyDevelopment(s *Settings) {
	s.Debug = true
	s.Logging.Level = "debug"
	s.Logging.Format = "text"
	s.Sessions.Secure = false
	if len(s.AllowedHosts) == 0 {
		s.AllowedHosts = []string{"127.0.0.1", "localhost", "[::1]"}
	}
}

func applyDeployed(s *Settings) {
	s.Debug = false
	s.Logging.Level = "info"
	s.Logging.Format = "json"
	s.Sessions.Secure = true
	if s.Server.ReadTimeout == 0 {
		s.Server.ReadTimeout = 15 * time.Second
	}
	if s.Server.WriteTimeout == 0 {
		s.Server.WriteTimeout = 30 * time.Second
	}
	if s.Server.ShutdownTimeout == 0 {
		s.Server.ShutdownTimeout = 20 * time.Second
	}
}

package settings

import "strings"

const minSecretKeyLength = 32

const missingSettingsMessage = `no settings have been configured

Coyote needs an explicit settings file. Create settings.go next to your main package:

	package main

	import "github.com/farhapartex/coyote/core/settings"

	func init() {
		settings.Configure(func(s *settings.Settings) {
			s.Debug = true
			s.SecretKey = "replace-me-with-32-or-more-random-characters"
			s.AllowedHosts = []string{"127.0.0.1", "localhost"}
			s.Templates.Dir = "templates"
		})
	}

Every setting not assigned there keeps its default from settings.Default(), including a
SQLite database at <project>/coyote.db as Databases[0].`

type ImproperlyConfigured struct {
	Problems []string
}

func (e *ImproperlyConfigured) Error() string {
	if len(e.Problems) == 1 {
		return "coyote/settings: improperly configured: " + e.Problems[0]
	}
	var b strings.Builder
	b.WriteString("coyote/settings: improperly configured:")
	for _, p := range e.Problems {
		b.WriteString("\n  - ")
		b.WriteString(p)
	}
	return b.String()
}

func (s Settings) validate() error {
	var problems []string
	add := func(problem string) { problems = append(problems, problem) }

	for _, check := range []func(func(string)){
		s.validateCore,
		s.validateDatabases,
		s.validateCaches,
		s.validatePageCache,
		s.validateI18N,
		s.validateServer,
		s.validateSessions,
		s.validateAuth,
		s.validateTemplates,
		s.validatePagination,
		s.validateUploads,
		s.validateStatic,
		s.validateAdmin,
		s.validateSecurity,
		s.validateLogging,
		s.validateEmail,
		s.validateJobs,
	} {
		check(add)
	}

	if len(problems) > 0 {
		return &ImproperlyConfigured{Problems: problems}
	}
	return nil
}

package settings

import "sync"

func New(fns ...func(*Settings)) (Settings, error) {
	s := Default()
	for _, fn := range fns {
		if fn != nil {
			fn(&s)
		}
	}
	if s.SecretKey == "" && s.Debug {
		key, err := generateSecretKey()
		if err != nil {
			return Settings{}, &ImproperlyConfigured{Problems: []string{
				"SecretKey is empty and a development key could not be generated: " + err.Error(),
			}}
		}
		s.SecretKey = key
		s.secretKeyGenerated = true
	}
	s.normalize()
	if err := s.validate(); err != nil {
		return Settings{}, err
	}
	return s, nil
}

var (
	mu         sync.RWMutex
	configured *Settings
)

func Configure(fns ...func(*Settings)) Settings {
	s, err := New(fns...)
	if err != nil {
		panic(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if configured != nil {
		panic(&ImproperlyConfigured{Problems: []string{
			"settings have already been configured; call settings.Configure exactly once, from settings.go",
		}})
	}
	configured = &s
	return s
}

func Get() Settings {
	mu.RLock()
	defer mu.RUnlock()
	if configured == nil {
		panic(&ImproperlyConfigured{Problems: []string{missingSettingsMessage}})
	}
	return *configured
}

func IsConfigured() bool {
	mu.RLock()
	defer mu.RUnlock()
	return configured != nil
}

func Reset() {
	mu.Lock()
	defer mu.Unlock()
	configured = nil
}

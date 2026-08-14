package settings

import (
	"crypto/tls"
	"time"
)

type TLS struct {
	CertFile   string
	KeyFile    string
	MinVersion uint16
	HSTS       time.Duration
	Config     *tls.Config
	Autocert   bool
	AcceptTOS  bool
	CacheDir   string
	Staging    bool
}

func (t TLS) Enabled() bool {
	if t.Config != nil || t.Autocert {
		return true
	}
	return t.CertFile != "" && t.KeyFile != ""
}

func (t TLS) Managed() bool { return t.Autocert && t.Config == nil }

func (t TLS) Scheme() string {
	if t.Enabled() {
		return "https"
	}
	return "http"
}

func (t TLS) Version() uint16 {
	if t.MinVersion != 0 {
		return t.MinVersion
	}
	return tls.VersionTLS12
}

func (t TLS) Build() *tls.Config {
	if t.Config != nil {
		config := t.Config.Clone()
		if config.MinVersion == 0 {
			config.MinVersion = t.Version()
		}
		return config
	}
	return &tls.Config{MinVersion: t.Version()}
}

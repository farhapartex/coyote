package certs

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"

	"github.com/farhapartex/coyote/core/settings"
	xacme "golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

const StagingDirectory = "https://acme-staging-v02.api.letsencrypt.org/directory"

var ErrNoHosts = errors.New("coyote/certs: autocert needs at least one host")

func Manager(cfg settings.TLS, hosts []string) (*autocert.Manager, error) {
	if len(hosts) == 0 {
		return nil, ErrNoHosts
	}
	if err := os.MkdirAll(cfg.CacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("coyote/certs: creating %s: %w", cfg.CacheDir, err)
	}

	manager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cfg.CacheDir),
		HostPolicy: autocert.HostWhitelist(hosts...),
	}
	if cfg.Staging {
		manager.Client = &xacme.Client{DirectoryURL: StagingDirectory}
	}
	return manager, nil
}

func TLSConfig(cfg settings.TLS, hosts []string) (*tls.Config, error) {
	if !cfg.Managed() {
		return cfg.Build(), nil
	}
	manager, err := Manager(cfg, hosts)
	if err != nil {
		return nil, err
	}
	config := manager.TLSConfig()
	config.MinVersion = cfg.Version()
	return config, nil
}

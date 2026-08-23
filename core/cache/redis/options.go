package redis

import (
	"crypto/tls"
	"time"
)

const (
	DefaultPoolSize  = 8
	DefaultTimeout   = 3 * time.Second
	DefaultScanCount = 500
	maxClearPasses   = 8
)

type Options struct {
	Address      string
	Username     string
	Password     string
	Database     int
	TLS          *tls.Config
	PoolSize     int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func (o Options) withDefaults() Options {
	if o.Address == "" {
		o.Address = "127.0.0.1:6379"
	}
	if o.PoolSize <= 0 {
		o.PoolSize = DefaultPoolSize
	}
	if o.DialTimeout <= 0 {
		o.DialTimeout = DefaultTimeout
	}
	if o.ReadTimeout <= 0 {
		o.ReadTimeout = DefaultTimeout
	}
	if o.WriteTimeout <= 0 {
		o.WriteTimeout = DefaultTimeout
	}
	return o
}

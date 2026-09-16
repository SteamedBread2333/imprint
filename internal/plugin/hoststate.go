package plugin

import (
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// StopBackgroundHost frees the host listen port for this project.
func StopBackgroundHost(cfg *Config) {
	if cfg == nil {
		return
	}
	listen := cfg.Host.Listen
	if listen == "" {
		listen = imprint.DefaultHostListen
	}
	_ = FreeListenPort(listen, 2*time.Second)
}

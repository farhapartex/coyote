package main

import (
	"github.com/farhapartex/coyote/core/auth"
	"github.com/farhapartex/coyote/core/settings"
)

func cacheBackend() settings.Cache {
	if address := settings.Env("REDIS_ADDR", ""); address != "" {
		return settings.Cache{
			Backend:  settings.CacheInRedis,
			Address:  address,
			Password: settings.Env("REDIS_PASSWORD", ""),
			Version:  settings.Env("DEPLOY_ID", "1"),
		}
	}
	return settings.MemoryCache("default")
}

func navKey(data map[string]any) string {
	path, _ := data["Path"].(string)
	who := "anonymous"
	if user, ok := data["User"].(*auth.User); ok && user != nil {
		who = user.Username
	}
	return path + "|" + who
}

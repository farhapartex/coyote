package settings

import (
	"fmt"
	"os"
)

type Source interface {
	Name() string
	Values() (map[string]string, error)
}

type Map map[string]string

func (m Map) Name() string { return "map" }

func (m Map) Values() (map[string]string, error) { return map[string]string(m), nil }

func Load(sources ...Source) error {
	for _, source := range sources {
		values, err := source.Values()
		if err != nil {
			return err
		}
		for key, value := range values {
			if _, present := os.LookupEnv(key); present {
				continue
			}
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("coyote/settings: %s: setting %s: %w", source.Name(), key, err)
			}
		}
	}
	return nil
}

func MustLoad(sources ...Source) {
	if err := Load(sources...); err != nil {
		panic(err)
	}
}

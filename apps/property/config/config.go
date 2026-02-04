package config

import (
	fconfig "github.com/pitabwire/frame/config"
)

type PropertyConfig struct {
	fconfig.ConfigurationDefault

	ProfileServiceURI   string `env:"PROFILE_SERVICE_URI" envDefault:"127.0.0.1:7005"`
	PartitionServiceURI string `env:"PARTITION_SERVICE_URI" envDefault:"127.0.0.1:7003"`
}

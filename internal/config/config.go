package config

import (
	"log"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	Port         string `env:"PORT" envDefault:"8080"`
	MongoURI     string `env:"MONGO_URI" envDefault:"mongodb://localhost:27017"`
	SitesFile    string `env:"SITES_FILE" envDefault:"configs/sites.json"`
}

func LoadConfig() *Config {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		log.Fatalf("Failed to load environment variables: %v", err)
	}
	return cfg
}
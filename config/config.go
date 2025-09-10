package config

import (
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Config struct {
	DBUrl  string
	AppEnv string // development, qa, production
	Port   int
}

func SetupConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Warn().Msg("Error to load the .env file")
	}

	// AutomaticEnv is a powerful helper especially when combined with SetEnvPrefix.
	// When called, Viper will check for an environment variable any time a viper.Get request is made.
	// It will apply the following rules. It will check for an environment variable with a name
	// matching the key uppercased and prefixed with the EnvPrefix if set.
	viper.AutomaticEnv()

	dbUrl := mustGetString("DATABASE_URL")
	port := mustGetInt("PORT")
	appEnv := mustGetString("APP_ENV")

	return &Config{
		DBUrl:  dbUrl,
		Port:   port,
		AppEnv: appEnv,
	}
}

func mustGetString(key string) string {
	v := viper.GetString(key)
	if v == "" {
		log.Fatal().Msgf("required config key '%s' is missing or empty", key)
	}

	return v
}

func mustGetInt(key string) int {
	v := viper.GetInt(key)
	if v == 0 {
		log.Fatal().Msgf("required config value for key '%s' is missing or empty", key)
	}

	return viper.GetInt(key)
}

package main

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type config struct {
	viper *viper.Viper
}

func initConfig(configFile string) (*config, error) {
	if configFile != "" && !fileExists(configFile) {
		return nil, fmt.Errorf("Config file '%s' does not exist", configFile)
	}
	conf := &config{
		viper: viper.New(),
	}
	conf.setConfigDefaults()
	if configFile != "" {
		conf.viper.SetConfigFile(configFile)
		if err := conf.viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("Fatal error in config file: %s", err)
		}
	}
	return conf, nil
}

func (conf *config) GetViper() *viper.Viper {
	return conf.viper
}

func (conf *config) setConfigDefaults() {
	// Broker
	conf.viper.SetDefault("broker.uri", "amqp://guest:guest@localhost:5672")
	conf.viper.SetDefault("service.name", "my-service")
	// Http service
	conf.viper.SetDefault("http.listen_to_hosts", []string{})
	conf.viper.SetDefault("http.listen_port", 8130)
	conf.viper.SetDefault("http.fwd_host", "http://localhost:8000/")
	conf.viper.SetDefault("http.fwd_port", 80)
	// Dashboard service
	conf.viper.SetDefault("dashboard.enabled", true)
	conf.viper.SetDefault("dashboard.listen_to_hosts", []string{})
	conf.viper.SetDefault("dashboard.listen_port", 18130)
	// Message
	conf.viper.SetDefault("message.receive_timeout", 10)
}

func fileExists(file string) bool {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}
	return true
}

// GetString gets the respective configuration as a string.
func (conf *config) GetString(key string) string {
	return conf.viper.GetString(key)
}

// GetInt gets the corresponding config for the key as an int.
func (conf *config) GetInt(key string) int {
	return conf.viper.GetInt(key)
}

// GetInt gets the corresponding config for the key as an bool.
func (conf *config) GetBool(key string) bool {
	return conf.viper.GetBool(key)
}

// GetStringSlice gets the config as a string slice.
func (conf *config) GetStringSlice(key string) []string {
	return conf.viper.GetStringSlice(key)
}

// IsSet gets the config as a string slice.
func (conf *config) IsSet(key string) bool {
	return conf.viper.IsSet(key)
}

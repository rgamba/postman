package main

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

type app struct {
	Version string
	Args    cliArgs
	Config  *config
}

type cliArgs struct {
	ConfigFile *string
	Verbose    *int
}

func createApp() app {
	return app{
		Version: "0.1",
		Args:    parseArgs(),
		Config:  nil,
	}
}

func (a *app) isVerbose1() bool {
	return *a.Args.Verbose == 1
}

func (a *app) isVerbose2() bool {
	return *a.Args.Verbose > 1
}

func (a *app) isVerbose3() bool {
	return *a.Args.Verbose > 2
}

func (a *app) validateConfigParams() {
	// These are the required config parameters.
	required := []string{
		"service.name",
	}
	errors := []string{}
	for _, key := range required {
		if !a.Config.IsSet(key) {
			errors = append(errors, key)
		}
	}
	if len(errors) > 0 {
		log.Fatalf("Missing the following configuration required parameters: %s", strings.Join(errors, ", "))
	}
}

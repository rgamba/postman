package main

import (
	"flag"
	"os"

	"github.com/rgamba/postman/async"

	log "github.com/sirupsen/logrus"
)

func main() {
	setLogConfig()
	// Create the cli app
	cli := createApp()
	log.Infof("Starting Postman v%s", cli.Version)
	var err error
	if cli.Config, err = initConfig(*cli.Args.ConfigFile); err != nil {
		log.Fatal(err)
	}
	cli.validateConfigParams()
	// Some info output...
	if cli.isVerbose2() {
		log.Info("Using configuration file ", *cli.Args.ConfigFile)
		log.Info("Service name: ", cli.Config.GetString("service.name"))
	}

	err = async.Connect(cli.Config.GetString("broker.uri"))
	if err != nil {
		log.Fatal(err)
	}
	if cli.isVerbose2() {
		log.Infof("Successfully connected to AMQP server on %s", cli.Config.GetString("broker.uri"))
	}
	defer async.Close()

	async.OnNewMessage = func(msg []byte) {
		log.Info("New message: ", string(msg))
	}

	forever := make(chan bool)
	<-forever
}

func setLogConfig() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func parseArgs() cliArgs {
	args := cliArgs{}
	args.ConfigFile = flag.String("config", "", "The configuration file to use")
	args.Verbose = flag.Int("v", 1, "The verbosity level [1-3]")
	flag.Parse()
	return args
}

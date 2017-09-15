package main

import (
	"flag"
	"os"

	"github.com/rgamba/postman/async"
	"github.com/rgamba/postman/async/protobuf"
	"github.com/rgamba/postman/proxy"

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
	if cli.isVerbose3() {
		log.SetLevel(log.DebugLevel)
	}
	// Some info output...
	if cli.isVerbose2() {
		log.Info("Using configuration file ", *cli.Args.ConfigFile)
		log.Info("Service name: ", cli.Config.GetString("service.name"))
	}

	err = async.Connect(cli.Config.GetString("broker.uri"), cli.Config.GetString("service.name"))
	if err != nil {
		log.Fatal(err)
	}
	if cli.isVerbose2() {
		log.Infof("Successfully connected to AMQP server on %s", cli.Config.GetString("broker.uri"))
	}
	defer async.Close()

	if cli.isVerbose3() {
		enableLogForRequestAndResponse(&cli)
	}

	// Start http proxy server
	proxy.StartHTTPServer(cli.Config.GetInt("http.listen_port"), cli.Config.GetString("http.fwd_host"))
	if cli.isVerbose2() {
		log.Infof("HTTP proxy server listening on 127.0.0.1:%d", cli.Config.GetInt("http.listen_port"))
	}
	c := make(chan bool)
	<-c
}

func enableLogForRequestAndResponse(a *app) {
	async.OnNewRequest = func(req protobuf.Request) {
		log.WithFields(log.Fields{
			"endpoint":   req.GetEndpoint(),
			"method":     req.GetMethod(),
			"request_id": req.GetId(),
		}).Debug("Incoming request")
	}
	async.OnNewResponse = func(resp protobuf.Response) {
		log.WithFields(log.Fields{
			"status_code": resp.GetStatusCode(),
			"request_id":  resp.GetRequestId(),
		}).Debug("Incoming response")
	}
}

func setLogConfig() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func parseArgs() cliArgs {
	args := cliArgs{}
	args.ConfigFile = flag.String("config", "", "The configuration file to use")
	args.Verbose = flag.Int("v", 1, "The verbosity level [1-3]")
	flag.Parse()
	return args
}

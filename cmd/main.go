package main

import (
	"flag"
	"os"

	"github.com/rgamba/postman/async"
	"github.com/rgamba/postman/async/protobuf"
	"github.com/rgamba/postman/dashboard"
	"github.com/rgamba/postman/proxy"
	"github.com/rgamba/postman/stats"

	log "github.com/sirupsen/logrus"
)

var (
	// Version of the current build. Set at build time.
	Version = "-"
	// Build is the git commit hash. Set at build time.
	Build = "-"
)

func main() {
	setLogConfig()
	// Create the cmd app
	cmd := createApp()
	log.Infof("Postman %s, Build: %s", Version, Build)
	var err error
	if cmd.Config, err = initConfig(*cmd.Args.ConfigFile); err != nil {
		log.Fatal(err)
	}
	cmd.validateConfigParams()
	if cmd.isVerbose3() {
		log.SetLevel(log.DebugLevel)
	}
	// Some info output...
	if cmd.isVerbose2() {
		log.Info("Using configuration file ", *cmd.Args.ConfigFile)
		log.Info("Service name: ", cmd.Config.GetString("service.name"))
	}

	err = async.Connect(cmd.Config.GetString("broker.uri"), cmd.Config.GetString("service.name"))
	if err != nil {
		log.Fatal(err)
	}
	if cmd.isVerbose2() {
		log.Infof("Successfully connected to AMQP server on %s", cmd.Config.GetString("broker.uri"))
	}
	defer async.Close()

	if cmd.isVerbose3() {
		enableLogForRequestAndResponse(&cmd)
	}

	// Start http proxy server
	proxy.StartHTTPServer(cmd.Config.GetInt("http.listen_port"), cmd.Config.GetString("http.fwd_host"))
	if cmd.isVerbose2() {
		log.Infof("HTTP proxy server listening on 127.0.0.1:%d", cmd.Config.GetInt("http.listen_port"))
	}

	// Start the dashboard service
	if cmd.Config.GetBool("dashboard.enabled") {
		dashboard.StartHTTPServer(cmd.Config.GetInt("dashboard.listen_port"), cmd.Config.GetViper())
		log.Infof("Dashboard HTTP server listening on 127.0.0.1:%d", cmd.Config.GetInt("dashboard.listen_port"))
	}

	// Stats module needs to purge data periodically.
	stats.AutoPurgeOldEvents()

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

func parseArgs() cmdArgs {
	args := cmdArgs{}
	args.ConfigFile = flag.String("config", "", "The configuration file to use")
	args.Verbose = flag.Int("v", 1, "The verbosity level [1-3]")
	flag.Parse()
	return args
}

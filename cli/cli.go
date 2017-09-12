package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rgamba/postman/async"
	"github.com/rgamba/postman/async/protobuf"

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

	go func() {
		time.Sleep(time.Second * 2)
		req := &protobuf.Request{
			Endpoint:      "/test",
			Headers:       []string{"header1", "header2"},
			Body:          "test body!",
			ResponseQueue: async.GetResponseQueueName(),
		}
		queuename := fmt.Sprintf("postman.req.%s", cli.Config.GetString("service.name"))
		log.Info("Sending request: ", queuename, req)
		async.SendRequestMessage(queuename, req, func(resp *protobuf.Response, err error) {
			if err != nil {
				log.Error(">> Response message error: ", err)
			} else {
			}
			log.Print(">> Message response: ", resp)
		})
	}()

	forever := make(chan bool)
	<-forever
}

func enableLogForRequestAndResponse(a *app) {
	async.OnNewRequest = func(msg []byte) {
		log.WithFields(log.Fields{
			"content": string(msg),
		}).Info("New incoming request")
	}
	async.OnNewResponse = func(msg []byte) {
		log.WithFields(log.Fields{
			"content": string(msg),
		}).Info("New incoming response")
	}
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

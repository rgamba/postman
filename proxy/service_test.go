package proxy

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/rgamba/postman/async"

	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	async.Connect("amqp://guest:guest@localhost:5672/", "test")
	defer async.Close()
	os.Exit(m.Run())
}

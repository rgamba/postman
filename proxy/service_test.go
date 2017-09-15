package proxy

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/rgamba/postman/async"

	log "github.com/sirupsen/logrus"
)

var httpServer *http.Server
var mockHttpServer *http.Server

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	async.Connect("amqp://guest:guest@localhost:5672/", "test")
	httpServer = StartHTTPServer(8081, "http://localhost:8082")
	defer async.Close()
	os.Exit(m.Run())
}

func _startMockServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello world\n")
	})
	mockHttpServer = &http.Server{
		Addr:    ":8082",
		Handler: mux,
	}
	go func() {
		if err := mockHttpServer.ListenAndServe(); err != nil {
			log.Fatalf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()
}

func _stopMockServer() {
	mockHttpServer.Close()
}

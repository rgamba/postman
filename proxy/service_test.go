package proxy

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/rgamba/postman/async"
	"github.com/rgamba/postman/async/protobuf"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var httpServer *http.Server
var mockServer *http.Server

const MockServerPort = 8083

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	mockServer = _createMockServer()
	forwardHost = fmt.Sprintf("http://localhost:%d", MockServerPort)
	async.Connect("amqp://guest:guest@localhost:5672/", "test")
	httpServer = StartHTTPServer(8081, forwardHost)
	defer async.Close()
	os.Exit(m.Run())
}

func TestCreateMockServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "service 2")
	})
	_createServer(mux, 8084)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d", MockServerPort))
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "hello world", string(body))

	resp, err = http.Get("http://localhost:8084")
	assert.Nil(t, err)

	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)
	assert.Equal(t, "service 2", string(body))
}

func TestForwardRequestCall(t *testing.T) {
	req := &protobuf.Request{Id: "1", Endpoint: "/one", Method: "GET", Headers: []string{"Content-Type: test"}, Body: "test"}
	resp, err := forwardRequestCall(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)
}

func TestForwardRequestCallErrorStatusCode(t *testing.T) {
	req := &protobuf.Request{Id: "1", Endpoint: "/notfound", Method: "GET"}
	resp, err := forwardRequestCall(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 404)
}

func TestForwardRequestCallWhenInvalidFwdHost(t *testing.T) {
	defer func() {
		forwardHost = fmt.Sprintf("http://localhost:%d", MockServerPort)
	}()
	forwardHost = fmt.Sprintf("http://localhost:8095") // Invalid port
	req := &protobuf.Request{Id: "1", Endpoint: "/notfound", Method: "GET"}
	resp, err := forwardRequestCall(req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestForwardRequestCallWhenFwdHostHasTrailingSlash(t *testing.T) {
	defer func() {
		forwardHost = fmt.Sprintf("http://localhost:%d", MockServerPort)
	}()
	forwardHost = fmt.Sprintf("http://localhost:%d/", MockServerPort)
	req := &protobuf.Request{Id: "1", Endpoint: "/one", Method: "GET", Headers: []string{"Content-Type: test"}, Body: "test"}
	resp, err := forwardRequestCall(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)
}

func TestConvertHTTPResponseToProtoResponse(t *testing.T) {
	req := &protobuf.Request{Id: "1", Endpoint: "/one", Method: "GET"}
	resp, err := forwardRequestCall(req)
	assert.NoError(t, err)
	protoresp, err := convertHTTPResponseToProtoResponse(resp)
	assert.NoError(t, err)
	assert.Equal(t, int(protoresp.StatusCode), resp.StatusCode)
	assert.Equal(t, protoresp.GetBody(), "one")
	assert.Equal(t, len(protoresp.Headers), len(resp.Header))
}

func TestConvertHTTPHeadersToSlice(t *testing.T) {
	headers := http.Header{
		"Content-Type": []string{"text/html", "charset=utf-8"},
	}
	newheaders := convertHTTPHeadersToSlice(headers)
	assert.Equal(t, len(newheaders), len(headers))
	assert.Equal(t, "Content-Type: text/html; charset=utf-8", newheaders[0])
}

func TestConvertHTTPHeadersToSliceSingle(t *testing.T) {
	headers := http.Header{
		"Content-Type": []string{"text/html"},
	}
	newheaders := convertHTTPHeadersToSlice(headers)
	assert.Equal(t, len(newheaders), len(headers))
	assert.Equal(t, "Content-Type: text/html", newheaders[0])
}

func TestConvertHTTPHeadersToSliceMultipleHeaders(t *testing.T) {
	headers := http.Header{
		"Content-Type":  []string{"text/html"},
		"Content-Type1": []string{"text/html"},
		"Content-Type2": []string{"text/html"},
	}
	newheaders := convertHTTPHeadersToSlice(headers)
	assert.Equal(t, len(newheaders), len(headers))
}

func TestForwardRequestAndCreateResponse(t *testing.T) {
	req := &protobuf.Request{Id: "1", Endpoint: "/one", Method: "GET", Headers: []string{"Content-Type: test"}, Body: "test"}
	resp, err := forwardRequestAndCreateResponse(req)
	assert.NoError(t, err)
	assert.Equal(t, req.Id, resp.RequestId)
	assert.Equal(t, 200, int(resp.StatusCode))
	assert.Equal(t, "one", resp.Body)
}

func TestGetServiceNameFromPath(t *testing.T) {
	values := [][]string{
		{"/my-service", "my-service"},
		{"/my-service/other/index.html", "my-service"},
		{"my-service/other/", "my-service"},
		{"my-service/other", "my-service"},
		{"/my service/other", "my service"},
		{"/MyService/other", "MyService"},
		{"", ""},
	}
	for i, v := range values {
		res := getServiceNameFromPath(v[0])
		assert.Equal(t, v[1], res, fmt.Sprintf("Line %d", i))
	}
}

func TestGetPathWithoutServiceName(t *testing.T) {
	values := [][]string{
		{"/my-service", "/"},
		{"/my-service/other/index.html", "/other/index.html"},
		{"my-service/other/", "/other/"},
		{"my-service/other", "/other"},
		{"/MyService/other", "/other"},
		{"", ""},
	}
	for i, v := range values {
		res := getPathWithoutServiceName(v[0])
		assert.Equal(t, v[1], res, fmt.Sprintf("Line %d", i))
	}
}

// Misc testing functions

func _createMockServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/one", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "one")
	})
	mux.HandleFunc("/two", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "two")
	})
	mux.HandleFunc("/notfound", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		io.WriteString(w, "notfound")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello world")
	})
	return _createServer(mux, MockServerPort)
}

func _createServer(mux *http.ServeMux, port int) *http.Server {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	go func() {
		server.ListenAndServe()
	}()
	return server
}

func _getRequestMockServer(path string) (string, error) {
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	url := fmt.Sprintf("http://localhost:%d%s", MockServerPort, path)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body), nil
}

func _postRequestMockServer(path string, values url.Values) (string, error) {
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	url := fmt.Sprintf("http://localhost:%d%s", MockServerPort, path)
	resp, err := http.PostForm(url, values)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body), nil
}

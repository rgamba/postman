package proxy

import (
	"bytes"
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
const TestServerPort = 8081

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	// Mockserver is the server that will simulate the microservice.
	mockServer = _createMockServer()
	forwardHost = fmt.Sprintf("http://localhost:%d", MockServerPort)
	async.Connect("amqp://guest:guest@localhost:5672/", "test")
	// Http server or testserver is the HTTP proxy server exposed by
	// postman.
	httpServer = StartHTTPServer(TestServerPort, forwardHost)
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
	assert.Equal(t, protoresp.Body, "one")
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

func TestServerDefaultHandlerWhenNoServiceNameIsProvided(t *testing.T) {
	_, statusCode, err := _getRequestTestServer("/")
	assert.Nil(t, err)
	assert.Equal(t, 400, statusCode)
}

func TestServerDefaultHandlerWhenInvalidServiceNameIsProvided(t *testing.T) {
	_, statusCode, err := _getRequestTestServer("/invalidservice/other")
	assert.Nil(t, err)
	assert.Equal(t, 400, statusCode)
}

func TestServerDefaultHandlerWhenTimeout(t *testing.T) {
	ch, _ := async.CreateNewChannel()
	_, err := ch.QueueDeclare(
		"postman.req.timeout", // Name
		false, // Durable
		false, // Delete when unused
		false, // Exclusive
		false, // No-wait
		nil,   // arguments
	)
	defer ch.QueueDelete("postman.req.timeout", false, false, true)
	defer ch.Close()
	_, statusCode, err := _getRequestTestServer("/timeout/other")
	assert.Nil(t, err)
	assert.Equal(t, 500, statusCode)
}

func TestServerDefaultHandlerOk(t *testing.T) {
	body, statusCode, err := _getRequestTestServer("/test/one")
	assert.Nil(t, err)
	assert.Equal(t, 200, statusCode)
	assert.Equal(t, "one", body)
}

func TestServerDefaultHandlerStatus404(t *testing.T) {
	body, statusCode, err := _getRequestTestServer("/test/notfound")
	assert.Nil(t, err)
	assert.Equal(t, 404, statusCode)
	assert.Equal(t, "notfound", body)
}

func TestServerDefaultHandlerHome(t *testing.T) {
	body, statusCode, err := _getRequestTestServer("/test")
	assert.Nil(t, err)
	assert.Equal(t, 200, statusCode)
	assert.Equal(t, "hello world", body)
}

func TestServerDiscardResponse(t *testing.T) {
	body, statusCode, err := _getRequestTestServerNoResponse("/test")
	assert.Nil(t, err)
	assert.Equal(t, 201, statusCode)
	assert.Equal(t, "", body)
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

func _getRequestServer(path string, port int) (string, int, error) {
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	url := fmt.Sprintf("http://localhost:%d%s", port, path)
	resp, err := http.Get(url)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body), resp.StatusCode, nil
}

func _getRequestServerNoResponse(path string, port int) (string, int, error) {
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	url := fmt.Sprintf("http://localhost:%d%s", port, path)
	resp, err := _getRequestServerWithHeaders(url, port, map[string]string{"Discard-Response": "Yes"}, "GET", "")
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body), resp.StatusCode, nil
}

func _getRequestServerWithHeaders(url string, port int, headers map[string]string, method string, reqBody string) (*http.Response, error) {
	client := &http.Client{}
	body := bytes.NewBuffer([]byte{})
	if reqBody != "" {
		body = bytes.NewBuffer([]byte(reqBody))
	}
	// Create request
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	// Add headers to request
	for name, val := range headers {
		request.Header.Add(name, val)
	}
	// Send and get the response
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func _getRequestMockServer(path string) (string, int, error) {
	return _getRequestServer(path, MockServerPort)
}

func _getRequestTestServer(path string) (string, int, error) {
	return _getRequestServer(path, TestServerPort)
}

func _getRequestTestServerNoResponse(path string) (string, int, error) {
	return _getRequestServerNoResponse(path, TestServerPort)
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

package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rgamba/postman/async"
	"github.com/rgamba/postman/async/protobuf"

	log "github.com/sirupsen/logrus"
)

var forwardHost string

// StartHTTPServer starts the new HTTP proxy service.
func StartHTTPServer(port int, forwardToHost string) error {
	forwardHost = forwardToHost
	async.ResponseMiddleware = forwardRequestAndCreateResponse

	http.HandleFunc("/_pm/multiple/", multipleCalls)
	http.HandleFunc("/", defaultHandler)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// Here we need to forward the request as an HTTP call to
// http.fwd_host which will normally be localhost.
func forwardRequestAndCreateResponse(req *protobuf.Request) (*protobuf.Response, error) {
	httpResponse, err := forwardRequestCall(req)
	if err != nil {
		return nil, err
	}
	resp, err := convertHTTPResponseToProtoResponse(httpResponse)
	if err != nil {
		return nil, err
	}
	resp.RequestId = req.GetId()
	return resp, nil
}

// Convert the proto.Request message to an HTTP request and send it through
// TODO: we should split this function in several smaller ones.
func forwardRequestCall(req *protobuf.Request) (*http.Response, error) {
	// Make request
	client := &http.Client{}
	endpoint := fmt.Sprintf("%s%s", forwardHost, req.GetEndpoint())
	// Create request body
	body := bytes.NewBuffer([]byte{})
	if req.GetBody() != "" {
		body = bytes.NewBuffer([]byte(req.GetBody()))
	}
	// Create request
	request, err := http.NewRequest(req.GetMethod(), endpoint, body)
	if err != nil {
		return nil, err
	}
	// Add headers to request
	for _, header := range req.GetHeaders() {
		parts := strings.Split(header, ":")
		request.Header.Add(parts[0], parts[1])
	}
	// Send and get the response
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func convertHTTPResponseToProtoResponse(response *http.Response) (*protobuf.Response, error) {
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	resp := &protobuf.Response{
		Body:       string(body),
		StatusCode: int32(response.StatusCode),
		Headers:    convertHTTPHeadersToSlice(response.Header),
	}
	return resp, nil
}

// We got an outgoing request. defaultHandler will marshall the http request
// and convert it to a protobuf.Response and then send it via the async package.
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	ch, err := async.CreateNewChannel()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.ToMap(),
		}).Warnf("Create channel error")
		sendJSON(w, err.ToMap(), http.StatusBadRequest)
		return
	}
	defer ch.Close()

	log.Debug("New outgoing request")
	body, _ := ioutil.ReadAll(r.Body)
	request := &protobuf.Request{
		Method:        r.Method,
		Headers:       convertHTTPHeadersToSlice(r.Header),
		Body:          string(body),
		Endpoint:      getPathWithoutServiceName(r.URL.Path),
		ResponseQueue: async.GetResponseQueueName(),
	}
	serviceName := getServiceNameFromPath(r.URL.Path)
	if serviceName == "" {
		// TODO: generalize and create a return error func
		http.Error(w, "{\"error\": \"invalid service name\"}", 404)
		return
	}
	// As the response is async we'll need to sync processes.
	c := make(chan bool)
	// Send the message via async and get back a response
	async.SendRequestMessage(ch, serviceName, request, func(resp *protobuf.Response, err *async.Error) {
		sendHTTPResponseFromProtobufResponse(w, resp, err)
		c <- true
	})
	// Wait for the response or timeout after 15 seconds.
	select {
	case <-c:
		// Pass
	case <-time.After(15 * time.Second):
		sendJSON(w, createResponseError("timeout"), http.StatusInternalServerError)
	}
}

func sendHTTPResponseFromProtobufResponse(w http.ResponseWriter, resp *protobuf.Response, err *async.Error) {
	// First check if we have any errors
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Warnf("Message response error")
		sendJSON(w, err.ToMap(), http.StatusBadRequest)
		return
	}
	// Add headers
	for _, header := range resp.GetHeaders() {
		parts := strings.Split(header, ":")
		w.Header().Set(parts[0], parts[1])
	}
	// Status code and body
	w.WriteHeader(int(resp.StatusCode))
	w.Write([]byte(resp.GetBody()))
}

func createResponseError(err interface{}) map[string]string {
	return map[string]string{
		"error": fmt.Sprintf("%s", err),
	}
}

func sendJSON(w http.ResponseWriter, arr interface{}, statusCode int) {
	content, err := json.Marshal(arr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendResponse(w, content, statusCode)
}

func sendResponse(w http.ResponseWriter, content []byte, statusCode int) {
	if content == nil {
		content = []byte{0x00}
	}
	contentLength := strconv.Itoa(len(content))
	w.Header().Set("Content-Length", contentLength)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Server", "Postman")

	w.WriteHeader(statusCode)
	w.Write(content)
}

func convertHTTPHeadersToSlice(head map[string][]string) []string {
	headers := []string{}
	for headerName, parts := range head {
		newHeader := fmt.Sprintf("%s: %s", headerName, strings.Join(parts, " "))
		headers = append(headers, newHeader)
	}
	return headers
}

func getServiceNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return ""
	}
	return parts[1]
}

func getPathWithoutServiceName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return ""
	}
	return "/" + strings.Join(parts[2:], "/")
}

func multipleCalls(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

package proxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/rgamba/postman/async"
	"github.com/rgamba/postman/async/protobuf"
	"github.com/rgamba/postman/lib"

	log "github.com/sirupsen/logrus"
)

var forwardHost string

// StartHTTPServer starts the new HTTP proxy service.
func StartHTTPServer(port int, forwardToHost string) *http.Server {
	forwardHost = forwardToHost
	async.ResponseMiddleware = forwardRequestAndCreateResponse

	mux := http.NewServeMux()
	mux.HandleFunc("/_postman/multiple/", multipleCalls)
	mux.HandleFunc("/", outgoingRequestHandler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()

	return srv
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
	resp.RequestId = req.Id
	return resp, nil
}

// Convert the proto.Request message to an HTTP request and send it through
// to forwardHost via HTTP which will normally live in the same host.
// TODO: we should split this function in several smaller ones.
func forwardRequestCall(req *protobuf.Request) (*http.Response, error) {
	// Make request
	client := &http.Client{}
	if forwardHost[len(forwardHost)-1] == '/' {
		forwardHost = forwardHost[:len(forwardHost)-1]
	}
	endpoint := fmt.Sprintf("%s%s", forwardHost, req.Endpoint)
	// Create request body
	body := bytes.NewBuffer([]byte{})
	if req.Body != "" {
		body = bytes.NewBuffer([]byte(req.Body))
	}
	// Create request
	request, err := http.NewRequest(req.Method, endpoint, body)
	if err != nil {
		return nil, err
	}
	// Add headers to request
	for _, header := range req.Headers {
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
// TODO: We need to break this large function apart.
func outgoingRequestHandler(w http.ResponseWriter, r *http.Request) {
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
		Service:       async.GetServiceName(),
	}
	serviceName := getServiceNameFromPath(r.URL.Path)
	if serviceName == "" {
		// TODO: generalize and create a return error func
		sendJSON(w, map[string]string{
			"error":   "invalid_parameters",
			"message": "service name is required",
		}, 400)
		return
	}
	// Check if the request needs a response or we can discard the response.
	if requestWantsToDiscardResponse(r) {
		// The request doesn't need us to wait for a response, then we'll just
		// send the response and send back a 201 - Created status code.
		err := async.SendMessageAndDiscardResponse(ch, serviceName, request)
		var resp *protobuf.Response
		if err == nil {
			resp = &protobuf.Response{StatusCode: 201, Body: ""}
		}
		sendHTTPResponseFromProtobufResponse(w, resp, err)
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

func requestWantsToDiscardResponse(request *http.Request) bool {
	header := getHeaderValue("Discard-Response", request.Header)
	return strings.ToUpper(header) == "YES"
}

func getHeaderValue(headerName string, headers http.Header) string {
	for header, values := range headers {
		if strings.ToUpper(header) == strings.ToUpper(headerName) {
			return strings.Join(values, "; ")
		}
	}
	return ""
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
	for _, header := range resp.Headers {
		parts := strings.Split(header, ":")
		w.Header().Set(parts[0], parts[1])
	}
	addRequestIDToHTTPResponse(w, resp)
	// Status code and body
	w.WriteHeader(int(resp.StatusCode))
	w.Write([]byte(resp.Body))
}

func addRequestIDToHTTPResponse(w http.ResponseWriter, resp *protobuf.Response) {
	w.Header().Set("Postman-Id", resp.RequestId)
}

func createResponseError(err interface{}) map[string]string {
	return map[string]string{
		"error": fmt.Sprintf("%s", err),
	}
}

func sendJSON(w http.ResponseWriter, arr interface{}, statusCode int) {
	lib.SendJSON(w, arr, statusCode)
}

func sendResponse(w http.ResponseWriter, content []byte, statusCode int) {
	lib.SendResponse(w, content, statusCode)
}

func convertHTTPHeadersToSlice(head map[string][]string) []string {
	headers := []string{}
	for headerName, parts := range head {
		newHeader := fmt.Sprintf("%s: %s", headerName, strings.Join(parts, "; "))
		headers = append(headers, newHeader)
	}
	return headers
}

func getServiceNameFromPath(path string) string {
	if path != "" && path[0] != '/' {
		path = "/" + path
	}
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return ""
	}
	return parts[1]
}

func getPathWithoutServiceName(path string) string {
	if path != "" && path[0] != '/' {
		path = "/" + path
	}
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

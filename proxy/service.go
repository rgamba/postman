package proxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/rgamba/postman/async"
	"github.com/rgamba/postman/async/protobuf"
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
	resp, err := convertHttpResponseToProtoResponse(httpResponse)
	if err != nil {
		return nil, err
	}
	resp.RequestId = req.GetId()
	return resp, nil
}

// Convert the proto.Request message to an HTTP request and send it through
func forwardRequestCall(req *protobuf.Request) (*http.Response, error) {
	// Make request
	client := &http.Client{}
	endpoint := fmt.Sprintf("%s%s", forwardHost, req.GetEndpoint())
	request, _ := http.NewRequest(req.GetMethod(), endpoint, nil)
	for _, header := range req.GetHeaders() {
		parts := strings.Split(header, ":")
		request.Header.Add(parts[0], parts[1])
	}
	request.Body.Write([]byte(req.GetBody()))
	// Get the response
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func convertHttpResponseToProtoResponse(response *http.Response) (*protobuf.Response, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	resp := &protobuf.Response{
		Body:       body,
		StatusCode: int32(response.StatusCode),
		Headers:    convertHttpHeadersFromSlice(response.Header),
	}
}

// We got an outgoing request. defaultHandler will marshall the http request
// and convert it to a protobuf.Response and then send it via the async package.
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	request := &protobuf.Request{
		Method:        r.Method,
		Headers:       convertHttpHeadersToSlice(r.Header),
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

	c := make(chan bool)

	async.SendRequestMessage(serviceName, request, func(resp *protobuf.Response, err error) {
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Errorf("Message response error")
			return
		}
		// Add headers
		for _, header := range resp.GetHeaders() {
			parts := strings.Split(header, ":")
			w.Header().Set(parts[0], parts[1])
		}
		w.WriteHeader(int(resp.StatusCode))
		w.Write([]byte(resp.GetBody()))
		c <- true
	})

	select {
	case <-c:
		// all Good
	case <-time.After(15 * time.Second):
		http.Error(w, "Timeout", http.StatusInternalServerError)
	}
}

func convertHttpHeadersFromSlice(head *http.Head) []string {
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

package proxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/rgamba/postman/async"
	"github.com/rgamba/postman/async/protobuf"
)

// StartHTTPServer starts the new HTTP proxy service.
func StartHTTPServer(port int) error {
	async.ResponseMiddleware = forwardRequestAndCreateResponse

	http.HandleFunc("/_pm/multiple/", multipleCalls)
	http.HandleFunc("/", defaultHandler)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// Here we need to forward the request as an HTTP call to
// http.fwd_host which will normally be localhost.
func forwardRequestAndCreateResponse(req *protobuf.Request) *protobuf.Response {
	// TODO: make a call to fwd_host here.
	return &protobuf.Response{
		Body:       req.Body + " response",
		StatusCode: 200,
	}
}

// We got an outgoing request. defaultHandler will marshall the http request
// and convert it to a protobuf.Response and then send it via the async package.
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	request := &protobuf.Request{
		Method:   r.Method,
		Headers:  getHeadersFromRequest(r),
		Body:     string(body),
		Endpoint: getPathWithoutServiceName(r.URL.Path),
	}
	serviceName := getServiceNameFromPath(r.URL.Path)

	async.SendRequestMessage(serviceName, request, func(resp *protobuf.Response, err error) {
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Errorf("Message response error")
		}
		// Add headers
		for _, header := range resp.GetHeaders() {
			parts := strings.Split(header, ":")
			w.Header().Add(parts[0], parts[1])
		}
		w.WriteHeader(int(resp.StatusCode))

		fmt.Fprintf(w, resp.GetBody())
	})
}

func getHeadersFromRequest(r *http.Request) []string {
	headers := []string{}
	for headerName, parts := range r.Header {
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

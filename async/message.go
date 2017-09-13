package async

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/rgamba/postman/async/protobuf"
	"github.com/twinj/uuid"
)

// ResponseMiddleware is the function that needs to be injected
// from an outside module.
// This will be passed a request and must respond a response.
var ResponseMiddleware func(*protobuf.Request) (*protobuf.Response, error)

// All requests we send out to AMQP server will be stored here so we
// can make a match when the message comes back.
// The hash key will be the request ID and the value will be the queue
// name where we need to send the response to.
var requests = map[string]*requestRecord{}

type requestRecord struct {
	request    *protobuf.Request
	onResponse func(*protobuf.Response, error)
}

// SendRequestMessage sends a new request message through
// the AMQP server to the appropriate
func SendRequestMessage(serviceName string, request *protobuf.Request, onResponse func(*protobuf.Response, error)) {
	queueName := fmt.Sprintf("postman.req.%s", serviceName)
	if request.GetId() == "" {
		uniqid := uuid.NewV4()
		request.Id = fmt.Sprintf("%s", uniqid)
	}
	req := &requestRecord{
		request:    request,
		onResponse: onResponse,
	}
	message, err := proto.Marshal(request)
	if err != nil {
		go onResponse(nil, err)
		return
	}
	err = sendMessageToQueue(message, queueName, true)
	if err != nil {
		fmt.Println(err)
		go onResponse(nil, err)
		return
	}
	appendRequest(req)
}

// This function gets executed when we get a new response
// out of the response queue. That's when the other end service
// processed our request and sent a response. We will try and match
// the response to the original request and execute the callback function.
func processMessageResponse(msg []byte) error {
	response := &protobuf.Response{}
	if err := proto.Unmarshal(msg, response); err != nil {
		return err
	}
	return matchResponseAndSendCallback(response)
}

// This gets executed when a new requests arrives in the request queue
// and our service instance gets to process it.
// We rely on ResponseMiddleware being injected with the appropriate logic
// to process the request and get a response.
func processMessageRequest(msg []byte) error {
	request := &protobuf.Request{}
	if err := proto.Unmarshal(msg, request); err != nil {
		return err
	}
	var response *protobuf.Response
	if ResponseMiddleware != nil {
		var err error
		response, err = ResponseMiddleware(request)
		if err != nil {
			return err
		}
	} else {
		response = &protobuf.Response{StatusCode: 501, RequestId: request.GetId()}
	}
	err := sendResponseMessage(request, response)
	return err
}

// When we're done processing the message and we already got a Response object
// we just marshall and send the message through the appropriate response queue.
func sendResponseMessage(request *protobuf.Request, response *protobuf.Response) error {
	message, err := proto.Marshal(response)
	if err != nil {
		return err
	}
	err = sendMessageToQueue(message, request.GetResponseQueue(), false)
	if err != nil {
		return err
	}
	return nil
}

// Try to match the response back to the original request that we issued.
// If a match is found (normally this will be the case), we'll call the callback
// function that was passed along with the original request.
func matchResponseAndSendCallback(response *protobuf.Response) error {
	requestRecord := getResponseRequest(response.GetRequestId())
	if requestRecord == nil {
		return fmt.Errorf("Unable to find matching request for '%s'", response.GetRequestId())
	}
	requestRecord.onResponse(response, nil)
	removeRequest(response.GetRequestId())
	return nil
}

func appendRequest(req *requestRecord) {
	mutex.Lock()
	requests[req.request.GetId()] = req
	mutex.Unlock()
}

func removeRequest(requestID string) {
	mutex.Lock()
	delete(requests, requestID)
	mutex.Unlock()
}

func getResponseRequest(requestID string) *requestRecord {
	mutex.Lock()
	val, ok := requests[requestID]
	mutex.Unlock()
	if !ok {
		return nil
	}
	return val
}

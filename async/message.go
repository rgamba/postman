package async

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/rgamba/postman/async/protobuf"
	"github.com/rgamba/postman/stats"
	"github.com/streadway/amqp"
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
	onResponse func(*protobuf.Response, *Error)
}

// SendRequestMessage sends a new request message through
// the AMQP server to the appropriate
func SendRequestMessage(ch *amqp.Channel, serviceName string, request *protobuf.Request, onResponse func(*protobuf.Response, *Error)) {
	queueName := buildRequestQueueName(serviceName)
	setRequestIDIfEmpty(request)
	if !queueExists(ch, queueName) {
		go onResponse(nil, createInvalidQueueNameError(queueName))
		return
	}
	// Encode message.
	message, _err := proto.Marshal(request)
	if _err != nil {
		go onResponse(nil, createError("unexpected", _err.Error(), nil))
		return
	}
	// Send it!
	err := publishMessage(ch, message, queueName)
	if err != nil {
		go onResponse(nil, err)
		return
	}
	// Save the request in the request queue.
	appendRequest(request, onResponse)
	go stats.RecordRequest(serviceName)
}

// SendMessageAndDiscardResponse does exactly the same as SendMessage but
// it doesn't expect to get a response in any way.
func SendMessageAndDiscardResponse(ch *amqp.Channel, serviceName string, request *protobuf.Request) *Error {
	request.ResponseQueue = "" // No response queue when we don't need response.
	queueName := buildRequestQueueName(serviceName)
	setRequestIDIfEmpty(request)
	if !queueExists(ch, queueName) {
		return createInvalidQueueNameError(queueName)
	}
	// Encode message.
	message, _err := proto.Marshal(request)
	if _err != nil {
		return createError("unexpected", _err.Error(), nil)
	}
	// Send it!
	err := publishMessage(ch, message, queueName)
	if err != nil {
		return err
	}
	go stats.RecordRequest(serviceName)
	return nil
}

// The queue name of the destination service.
// Don't confuse this one for getRequestQueueName().
func buildRequestQueueName(serviceName string) string {
	return fmt.Sprintf("postman.req.%s", serviceName)
}

func setRequestIDIfEmpty(request *protobuf.Request) {
	if request.GetId() == "" {
		uniqid := uuid.NewV4()
		request.Id = fmt.Sprintf("%s", uniqid)
	}
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
	if OnNewResponse != nil {
		go OnNewResponse(*response)
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
	// Log?
	if OnNewRequest != nil {
		go OnNewRequest(*request)
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
	// We'll send a response only if we have a response queue name
	// if we don't have a queue, then it means we don't need to send a response back.
	if request.GetResponseQueue() != "" {
		err := sendResponseMessage(request, response)
		return err
	}
	return nil
}

// When we're done processing the message and we already got a Response object
// we just marshall and send the message through the appropriate response queue.
func sendResponseMessage(request *protobuf.Request, response *protobuf.Response) error {
	// TODO: should we reuse a channel or create a new one?
	ch, _err := CreateNewChannel()
	if _err != nil {
		return _err
	}
	defer ch.Close()
	// Encode response struct.
	message, err := proto.Marshal(response)
	if err != nil {
		return err
	}
	// Send through the response queue.
	_err = publishMessage(ch, message, request.GetResponseQueue())
	if _err != nil {
		return _err
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

func appendRequest(request *protobuf.Request, onResponse func(*protobuf.Response, *Error)) {
	req := &requestRecord{
		request:    request,
		onResponse: onResponse,
	}
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

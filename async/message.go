package async

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/rgamba/postman/async/protobuf"
	"github.com/twinj/uuid"
)

// All requests we send out to AMQP server
// will be stored here so we can make a match
// when the message comes back.
// The hash key will be the request ID and the
// value will be the queue name where we need
// to send the response to.
var requests = map[string]*requestRecord{}

type requestRecord struct {
	request    *protobuf.Request
	onResponse func(*protobuf.Response, error)
}

// SendRequestMessage sends a new request message through
// the AMQP server to the appropriate
func SendRequestMessage(queueName string, request *protobuf.Request, onResponse func(*protobuf.Response, error)) {
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
		onResponse(nil, err)
		return
	}
	err = sendMessageToQueue(message, queueName)
	if err != nil {
		onResponse(nil, err)
		return
	}
	appendRequest(req)
}

func processMessageResponse(msg []byte) error {
	response := &protobuf.Response{}
	if err := proto.Unmarshal(msg, response); err != nil {
		return err
	}
	return matchResponseAndSendCallback(response)
}

func processMessageRequest(msg []byte) error {
	request := &protobuf.Request{}
	if err := proto.Unmarshal(msg, request); err != nil {
		return err
	}
	response := &protobuf.Response{
		Body:      request.Body + " RESPONSE",
		RequestId: request.GetId(),
		Headers:   []string{"responseheader1", "responseHeader2"},
	}
	err := sendResponseMessage(request, response)
	return err
}

func sendResponseMessage(request *protobuf.Request, response *protobuf.Response) error {
	message, err := proto.Marshal(response)
	if err != nil {
		return err
	}
	err = sendMessageToQueue(message, request.GetResponseQueue())
	if err != nil {
		return err
	}
	return nil
}

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

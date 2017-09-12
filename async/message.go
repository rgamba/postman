package async

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/twinj/uuid"

	"github.com/rgamba/postman/async/protobuf"
)

// All requests we send out to AMQP server
// will be stored here so we can make a match
// when the message comes back.
// The hash key will be the request ID and the
// value will be the queue name where we need
// to send the response to.
var requests map[string]*requestRecord

type requestRecord struct {
	request    *protobuf.Request
	onResponse func(*protobuf.Response, error)
}

// SendMessage sends a new request message through
// the AMQP server to the appropriate
func SendMessage(request *protobuf.Request, onResponse func(*protobuf.Response, error)) {
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
	err = sendMessageToQueue(message, request.GetResponseQueue())
	if err != nil {
		onResponse(nil, err)
		return
	}
	appendRequest(req)
}

func processMessageResponse(msg []byte) {

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

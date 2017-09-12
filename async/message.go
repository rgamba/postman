package async

import "sync"

// All requests we send out to AMQP server
// will be stored here so we can make a match
// when the message comes back.
// The hash key will be the request ID and the
// value will be the queue name where we need
// to send the response to.
var requests map[string]string
var mutex = &sync.Mutex{}

func processMessageResponse(msg []byte) {

}

func appendRequest(requestID string, queueID string) {
	mutex.Lock()
	requests[requestID] = queueID
	mutex.Unlock()
}

func removeRequest(requestID string) {
	mutex.Lock()
	delete(requests, requestID)
	mutex.Unlock()
}

func getResponseQueue(requestID string) string {
	mutex.Lock()
	val, ok := requests[requestID]
	mutex.Unlock()
	if !ok {
		return ""
	}
	return val
}

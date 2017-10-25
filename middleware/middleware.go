// Middleware package
//
// Middlewares are add-ons that can be hooked in various
// steps of the request/response process. Let's look at a
// practical example.
//
// Assume we have 2 services:
// Service A and Service B. Service A needs to call service B
// in this case the order of middleware events will be:
//
// 1. OutgoingRequest relative to service A
// 2. IncomingRequest relative to service B
// 3. IncomingResponse relative to service B
// 4. OutgoingResponse relative to service A

package middleware

import "github.com/rgamba/postman/async/protobuf"

type requestMiddleware struct {
	side    string
	handler func(*protobuf.Request)
}

type responseMiddleware struct {
	side    string
	handler func(*protobuf.Response)
}

var (
	requestMiddlewares  []*requestMiddleware
	responseMiddlewares []*responseMiddleware
)

const (
	SideOutgoing = "outgoing"
	SideIncoming = "incoming"
)

// RegisterOutgoingRequestMiddleware happens after postman receives a new outgoing request from
// the local service and encodes it in the postman format.
// Once the request is encoded, all middleware will be processed before sending the request over
// to the receiving requests destination queue.
func RegisterOutgoingRequestMiddleware(handler func(*protobuf.Request)) {
	registerRequestMiddleware(SideOutgoing, handler)
}

// RegisterIncomingRequestMiddleware registers a new middleware that happens when
// postman receives a new incoming request from the request queue, then decodes the message
// and any middleware registered will take that request and transform it if necessary before
// sending it forward to the receiver service as an http call.
func RegisterIncomingRequestMiddleware(handler func(*protobuf.Request)) {
	registerRequestMiddleware(SideIncoming, handler)
}

// RegisterIncomingResponseMiddleware registers a new middleware that will be triggered after
// the IncomingRequest, when the local service has processed the request and sent a response,
// postman catches that response, encodes it and then all middleware will be run.
func RegisterIncomingResponseMiddleware(handler func(*protobuf.Response)) {
	registerResponseMiddleware(SideIncoming, handler)
}

// RegisterOutgoingResponseMiddleware registers a middleware that will be triggered when the response
// for an OutgoingRequest arrives back to postman.
func RegisterOutgoingResponseMiddleware(handler func(*protobuf.Response)) {
	registerResponseMiddleware(SideOutgoing, handler)
}

func registerRequestMiddleware(side string, handler func(*protobuf.Request)) {
	requestMiddlewares = append(requestMiddlewares, &requestMiddleware{
		side:    side,
		handler: handler,
	})
}

func registerResponseMiddleware(side string, handler func(*protobuf.Response)) {
	responseMiddlewares = append(responseMiddlewares, &responseMiddleware{
		side:    side,
		handler: handler,
	})
}

func ProcessIncomingRequestMiddlewares(request *protobuf.Request) {
	processRequestMiddlewares(SideIncoming, request)
}

func ProcessOutgoingRequestMiddlewares(request *protobuf.Request) {
	processRequestMiddlewares(SideOutgoing, request)
}

func ProcessIncomingResponseMiddlewares(response *protobuf.Response) {
	processResponseMiddlewares(SideIncoming, response)
}

func ProcessOutgoingResponseMiddlewares(response *protobuf.Response) {
	processResponseMiddlewares(SideOutgoing, response)
}

func processRequestMiddlewares(side string, request *protobuf.Request) {
	for _, mid := range requestMiddlewares {
		if mid.side == side {
			mid.handler(request)
		}
	}
}

func processResponseMiddlewares(side string, response *protobuf.Response) {
	for _, mid := range responseMiddlewares {
		if mid.side == side {
			mid.handler(response)
		}
	}
}

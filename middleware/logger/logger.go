package logger

import (
	"github.com/rgamba/postman/async/protobuf"
	"github.com/rgamba/postman/middleware"

	log "github.com/sirupsen/logrus"
)

func Init() {
	middleware.RegisterIncomingRequestMiddleware(func(req *protobuf.Request) {
		log.WithFields(log.Fields{
			"endpoint":   req.Endpoint,
			"method":     req.Method,
			"request_id": req.Id,
			"requester":  req.Service,
		}).Debug("Incoming request")
	})
	middleware.RegisterIncomingResponseMiddleware(func(resp *protobuf.Response) {
		log.WithFields(log.Fields{
			"status_code": resp.StatusCode,
			"request_id":  resp.RequestId,
		}).Debug("Incoming response")
	})
	middleware.RegisterOutgoingRequestMiddleware(func(req *protobuf.Request) {
		log.WithFields(log.Fields{
			"endpoint":   req.Endpoint,
			"method":     req.Method,
			"request_id": req.Id,
		}).Debug("Outgoing request")
	})
	middleware.RegisterOutgoingResponseMiddleware(func(resp *protobuf.Response) {
		log.WithFields(log.Fields{
			"status_code": resp.StatusCode,
			"request_id":  resp.RequestId,
		}).Debug("Outgoing response")
	})
}

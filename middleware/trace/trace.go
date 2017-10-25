package trace

import (
	"github.com/rgamba/postman/async/protobuf"
	"github.com/rgamba/postman/middleware"
	"github.com/rgamba/postman/stats"
)

func Init() {
	middleware.RegisterIncomingRequestMiddleware(func(req *protobuf.Request) *protobuf.Request {
		go stats.RecordRequest(req.Service, stats.Outgoing)
		return req
	})
}

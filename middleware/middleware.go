package middleware

import "github.com/rgamba/postman/async/protobuf"

type RequestMiddleware struct {
	event   string
	handler func(*protobuf.Request) (*protobuf.Request, error)
}

type RequestMiddleware func(*protobuf.Request) (*protobuf.Request, error)
type ResponseMiddleware func(*protobuf.Response) (*protobuf.Response, error)

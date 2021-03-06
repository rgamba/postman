// Code generated by protoc-gen-go.
// source: request.proto
// DO NOT EDIT!

/*
Package protobuf is a generated protocol buffer package.

It is generated from these files:
	request.proto
	response.proto

It has these top-level messages:
	Request
	Response
*/
package protobuf

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
const _ = proto.ProtoPackageIsVersion1

type Request struct {
	Id            string   `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	Method        string   `protobuf:"bytes,2,opt,name=method" json:"method,omitempty"`
	Endpoint      string   `protobuf:"bytes,3,opt,name=endpoint" json:"endpoint,omitempty"`
	Headers       []string `protobuf:"bytes,4,rep,name=headers" json:"headers,omitempty"`
	ResponseQueue string   `protobuf:"bytes,5,opt,name=response_queue,json=responseQueue" json:"response_queue,omitempty"`
	Body          string   `protobuf:"bytes,6,opt,name=body" json:"body,omitempty"`
	Service       string   `protobuf:"bytes,7,opt,name=service" json:"service,omitempty"`
}

func (m *Request) Reset()                    { *m = Request{} }
func (m *Request) String() string            { return proto.CompactTextString(m) }
func (*Request) ProtoMessage()               {}
func (*Request) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func init() {
	proto.RegisterType((*Request)(nil), "protobuf.Request")
}

var fileDescriptor0 = []byte{
	// 177 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x3c, 0x8f, 0x4b, 0x0a, 0xc2, 0x40,
	0x0c, 0x40, 0xe9, 0xc7, 0x7e, 0x02, 0xed, 0x22, 0x0b, 0x09, 0xae, 0x8a, 0x20, 0x74, 0xe5, 0xc6,
	0x93, 0xd8, 0x0b, 0x88, 0x75, 0x22, 0x9d, 0x85, 0x9d, 0x76, 0x3e, 0x82, 0x77, 0xf3, 0x70, 0xd2,
	0x68, 0x5d, 0xcd, 0xbc, 0xf7, 0x48, 0x20, 0x50, 0x59, 0x9e, 0x03, 0x3b, 0x7f, 0x9c, 0xac, 0xf1,
	0x06, 0x0b, 0x79, 0xfa, 0x70, 0xdf, 0xbf, 0x23, 0xc8, 0xbb, 0x6f, 0xc3, 0x1a, 0x62, 0xad, 0x28,
	0x6a, 0xa2, 0xb6, 0xec, 0x62, 0xad, 0x70, 0x0b, 0xd9, 0x83, 0xfd, 0x60, 0x14, 0xc5, 0xe2, 0x7e,
	0x84, 0x3b, 0x28, 0x78, 0x54, 0x93, 0xd1, 0xa3, 0xa7, 0x44, 0xca, 0x9f, 0x91, 0x20, 0x1f, 0xf8,
	0xaa, 0xd8, 0x3a, 0x4a, 0x9b, 0xa4, 0x2d, 0xbb, 0x15, 0xf1, 0x00, 0xb5, 0x65, 0x37, 0x99, 0xd1,
	0xf1, 0x65, 0x0e, 0x1c, 0x98, 0x36, 0x32, 0x5b, 0xad, 0xf6, 0xbc, 0x48, 0x44, 0x48, 0x7b, 0xa3,
	0x5e, 0x94, 0x49, 0x94, 0xff, 0xb2, 0xd4, 0xb1, 0x7d, 0xea, 0x1b, 0x53, 0x2e, 0x7a, 0xc5, 0x3e,
	0x93, 0x43, 0x4e, 0x9f, 0x00, 0x00, 0x00, 0xff, 0xff, 0x3a, 0xc4, 0xfc, 0x6a, 0xe0, 0x00, 0x00,
	0x00,
}

package async

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/rgamba/postman/async/protobuf"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestConnectInvalid(t *testing.T) {
	err := Connect("amqp://guest:guest@localhost:56721", "test-service")
	if err == nil {
		t.Error("Expected connection error")
	}
}

func TestRequestQueueName(t *testing.T) {
	if getRequestQueueName() != "postman.req.test-service" {
		t.Error("Invalid getRequestQueueName")
	}
}

func TestEnsureRequestQueue(t *testing.T) {
	_connect()
	defer _close_connection()
	if !_queueExists(getRequestQueueName()) {
		t.Error("Request queue does not exist")
	}
}

func TestResponseQueueCreation(t *testing.T) {
	_connect()
	defer _close_connection()
	if responseQueueName == "" {
		t.Error("Response queue name is empty")
	}
	if responseChannel == nil {
		t.Error("Response channel is nil")
	}
	if !_queueExists(responseQueueName) {
		t.Error("Response queue was not created or not created with the appropriate name")
	}
}

func TestConsumeFromRequestQueue(t *testing.T) {
	_connect()
	defer _close_connection()
	c := make(chan bool)
	OnNewRequest = func(req protobuf.Request) {
		if req.GetBody() != "test" {
			t.Errorf("Expected 'test' received '%s'", req.GetBody())
		}
		c <- true
	}
	request := &protobuf.Request{Body: "test"}
	msg, _ := proto.Marshal(request)
	_publishMessage(msg, getRequestQueueName())
	<-c
}

func TestConsumeFromResponseQueue(t *testing.T) {
	_connect()
	defer _close_connection()
	c := make(chan bool)
	OnNewResponse = func(resp protobuf.Response) {
		c <- true
	}
	response := &protobuf.Response{Body: "test"}
	msg, _ := proto.Marshal(response)
	_publishMessage(msg, responseQueueName)
	<-c
}

func TestQueueExists(t *testing.T) {
	_connect()
	defer _close_connection()
	ch, _ := conn.Channel()
	if exists := queueExists(ch, getRequestQueueName()); !exists {
		t.Errorf("Unexpected value for queueExists")
	}
}

func TestPublishMessage(t *testing.T) {
	_connect()
	defer _close_connection()
	resp := make(chan []byte)
	_createQueue("test")
	_consumeQueue("test", func(msg []byte) {
		resp <- msg
	})
	ch, _ := conn.Channel()
	err := publishMessage(ch, []byte("test"), "test")
	if err != nil {
		t.Error("Unexpected error: ", err.Error())
	}
	if string(<-resp) != "test" {
		t.Error("Unexpected response")
	}
}

func TestSendMessage(t *testing.T) {
	_connect()
	defer _close_connection()
	_createQueue("postman.req.service1")
	_consumeQueue("postman.req.service1", func(msg []byte) {
		req := &protobuf.Request{}
		proto.Unmarshal(msg, req)
		response := &protobuf.Response{Body: "testresponse", RequestId: req.GetId()}
		respMsg, _ := proto.Marshal(response)
		_publishMessage(respMsg, req.GetResponseQueue())
	})
	c := make(chan bool)
	ch, _ := conn.Channel()
	req := &protobuf.Request{Body: "test", Method: "GET", ResponseQueue: responseQueueName}
	SendRequestMessage(ch, "service1", req, func(resp *protobuf.Response, err *Error) {
		if err != nil {
			t.Errorf("Unexpected error: %s", err.Error())
		}
		if resp.GetBody() != "testresponse" {
			t.Errorf("Expected response 'testresponse' and got '%s'", resp.GetBody())
		}
		c <- true
	})
	<-c
}

func TestSendMessageParallelCalls(t *testing.T) {
	_connect()
	defer _close_connection()
	_createQueue("postman.req.service1")
	_consumeQueue("postman.req.service1", func(msg []byte) {
		req := &protobuf.Request{}
		proto.Unmarshal(msg, req)
		response := &protobuf.Response{Body: req.GetBody(), RequestId: req.GetId()}
		respMsg, _ := proto.Marshal(response)
		_publishMessage(respMsg, req.GetResponseQueue())
	})
	var wait sync.WaitGroup
	for i := 0; i <= 100; i++ {
		go func(i int) {
			wait.Add(1)
			req := &protobuf.Request{Body: fmt.Sprintf("%d", i), Method: "GET", ResponseQueue: responseQueueName}
			ch, _ := conn.Channel()
			SendRequestMessage(ch, "service1", req, func(resp *protobuf.Response, err *Error) {
				expectedBody := fmt.Sprintf("%d", i)
				if err != nil {
					t.Errorf("Unexpected error: %s", err.Error())
				}
				if resp.GetBody() != expectedBody {
					t.Errorf("Expected response '%s' and got '%s' instead", expectedBody, resp.GetBody())
				}
				wait.Done()
			})
		}(i)
	}
	wait.Wait()
}

func TestSendMessageWhenQueueDoesntExist(t *testing.T) {
	_connect()
	defer _close_connection()
	c := make(chan bool)
	ch, _ := conn.Channel()
	req := &protobuf.Request{Body: "test", Method: "GET", ResponseQueue: responseQueueName}
	SendRequestMessage(ch, "service1", req, func(resp *protobuf.Response, err *Error) {
		if err == nil {
			t.Errorf("Expected invalid queue error")
		}
		c <- true
	})
	<-c
}

func TestSendMessageWhenClosedChannel(t *testing.T) {
	_connect()
	defer _close_connection()
	c := make(chan bool)
	ch, _ := conn.Channel()
	ch.Close()
	req := &protobuf.Request{Body: "test", Method: "GET", ResponseQueue: responseQueueName}
	SendRequestMessage(ch, "service1", req, func(resp *protobuf.Response, err *Error) {
		if err == nil {
			t.Errorf("Expected invalid queue error")
		}
		c <- true
	})
	<-c
}

// Misc functions

func _queueExists(queueName string) bool {
	ch, _ := conn.Channel()
	_, err := ch.QueueInspect(queueName)
	if err == nil {
		return true
	}
	return false
}

func _connect() {
	Connect("amqp://guest:guest@localhost:5672", "test-service")
}

func _close_connection() {
	Close()
}

func _publishMessage(message []byte, queueName string) error {
	ch, _ := conn.Channel()
	err := ch.Publish(
		"", // Exchange, we don't use exchange
		queueName,
		false, // Mandatory
		false, // Immediate?
		amqp.Publishing{
			ContentType:  "application/octet-stream",
			Body:         message,
			DeliveryMode: amqp.Persistent,
		},
	)
	return err
}

func _consumeQueue(queueName string, fn func([]byte)) {
	msgs, _ := responseChannel.Consume(
		queueName, // Queue name
		"",        // Consumer
		true,      // Auto ack
		false,     // Exclusive
		false,     // No-local
		false,     // No-wait
		nil,       // args
	)
	go func() {
		for d := range msgs {
			fn(d.Body)
		}
	}()
}

func _createQueue(name string) {
	ch, _ := conn.Channel()
	ch.QueueDeclare(
		name,  // Name
		true,  // Durable
		true,  // Delete when unused
		false, // Exclusive
		false, // No-wait
		nil,   // arguments
	)
}

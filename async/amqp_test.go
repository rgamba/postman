package async

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/rgamba/postman/async/protobuf"
	log "github.com/sirupsen/logrus"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestRequestQueueName(t *testing.T) {
	_connect()
	defer _close_connection()
	assert.Equal(t, getRequestQueueName(), "postman.req.test-service")
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
	assert.NotEmpty(t, responseQueueName)
	assert.NotNil(t, responseChannel)
	assert.True(t, _queueExists(responseQueueName))
}

func TestConsumeFromRequestQueue(t *testing.T) {
	_connect()
	defer _close_connection()
	c := make(chan bool)
	OnNewRequest = func(req protobuf.Request) {
		assert.Equal(t, "test", req.Body)
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
	assert.Nil(t, err)
	assert.Equal(t, string(<-resp), "test")
}

func TestSendMessage(t *testing.T) {
	_connect()
	defer _close_connection()
	_createQueue("postman.req.service1")
	_consumeQueue("postman.req.service1", func(msg []byte) {
		req := &protobuf.Request{}
		proto.Unmarshal(msg, req)
		response := &protobuf.Response{Body: "testresponse", RequestId: req.Id}
		respMsg, _ := proto.Marshal(response)
		_publishMessage(respMsg, req.ResponseQueue)
	})
	c := make(chan bool)
	ch, _ := conn.Channel()
	req := &protobuf.Request{Body: "test", Method: "GET", ResponseQueue: responseQueueName}
	SendRequestMessage(ch, "service1", req, func(resp *protobuf.Response, err *Error) {
		assert.Nil(t, err)
		assert.Equal(t, "testresponse", resp.Body)
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
		response := &protobuf.Response{Body: req.Body, RequestId: req.Id}
		respMsg, _ := proto.Marshal(response)
		_publishMessage(respMsg, req.ResponseQueue)
	})
	var wait sync.WaitGroup
	for i := 0; i <= 100; i++ {
		go func(i int) {
			wait.Add(1)
			req := &protobuf.Request{Body: fmt.Sprintf("%d", i), Method: "GET", ResponseQueue: responseQueueName}
			ch, _ := conn.Channel()
			SendRequestMessage(ch, "service1", req, func(resp *protobuf.Response, err *Error) {
				expectedBody := fmt.Sprintf("%d", i)
				assert.Nil(t, err)
				assert.Equal(t, expectedBody, resp.Body)
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
		assert.NotNil(t, err)
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
		assert.NotNil(t, err)
		c <- true
	})
	<-c
}

func TestSendMessageAndDiscardResponse(t *testing.T) {
	_connect()
	defer _close_connection()
	_createQueue("postman.req.service1")
	ch, _ := conn.Channel()
	req := &protobuf.Request{Body: "test", Method: "GET", ResponseQueue: "responsequeue"}
	err := SendMessageAndDiscardResponse(ch, "service1", req)
	assert.Nil(t, err)
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
	time.Sleep(time.Millisecond * 100)
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

package async

import (
	"os"
	"testing"

	"github.com/streadway/amqp"
)

func TestMain(m *testing.M) {
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
	if !_queueExists(getRequestQueueName()) {
		t.Error("Request queue does not exist")
	}
}

func TestResponseQueueCreation(t *testing.T) {
	_connect()
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
	c := make(chan bool)
	OnNewRequest = func(msg []byte) {
		if string(msg) != "test" {
			t.Errorf("Expected 'test' received '%s'", msg)
		}
		c <- true
	}
	_publishMessage([]byte("test"), getRequestQueueName())
	<-c
}

func TestConsumeFromResponseQueue(t *testing.T) {
	_connect()
	c := make(chan bool)
	OnNewResponse = func(msg []byte) {
		if string(msg) != "test" {
			t.Errorf("Expected 'test' received '%s'", msg)
		}
		c <- true
	}
	_publishMessage([]byte("test"), responseQueueName)
	<-c
}

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

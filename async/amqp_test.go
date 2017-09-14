package async

import (
	"io/ioutil"
	"os"
	"testing"

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

func TestCacheSendChannel(t *testing.T) {
	_connect()
	cacheSendChannel("test", nil)
	ch, ok := sendChannels["test"]
	if !ok {
		t.Errorf("Unable to get 'test' sendChannel")
	}
	if ch != nil {
		t.Errorf("Inconrrect channel value")
	}
}

func TestDeleteSendChannelCache(t *testing.T) {
	_connect()
	sendChannels["test"] = nil
	deleteSendChannelCache("test")
	_, ok := sendChannels["test"]
	if ok {
		t.Errorf("Unable to delete 'test' sendChannel")
	}
}

func TestQueueExists(t *testing.T) {
	_connect()
	ch, _ := conn.Channel()
	if exists := queueExists(ch, getRequestQueueName()); !exists {
		t.Errorf("Unexpected value for queueExists")
	}
}

func TestPublishMessage(t *testing.T) {
	_connect()
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

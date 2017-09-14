package async

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rgamba/postman/async/protobuf"
	"github.com/satori/go.uuid"
	"github.com/streadway/amqp"

	log "github.com/sirupsen/logrus"
)

// OnNewResponse execute this method each time a new
// message response gets to the response queue.
var OnNewResponse func(protobuf.Response)

// OnNewRequest will get executed each time a new requests
// gets delivered to our instance.
var OnNewRequest func(protobuf.Request)

// We'll use only one connection, this is the one.
var conn *amqp.Connection

// These channels are only used for new message consuming.
var responseChannel *amqp.Channel
var requestChannel *amqp.Channel
var mutex = &sync.Mutex{}

// The queue name we'll consume the response messages from.
var responseQueueName string

// We'll store the service name here.
var serviceName string

// Connect starts the connection to the AMQP server.
func Connect(uri string, service string) error {
	serviceName = service
	var err error
	conn, err = amqp.Dial(uri)
	if err != nil {
		return fmt.Errorf("Unable to connect to the AMQP server: %s", err)
	}
	err = declareResponseChannelAndQueue()
	if err != nil {
		return fmt.Errorf("Error creating the AMQP response channel: %s", err)
	}
	err = ensureRequestQueue()
	if err != nil {
		return fmt.Errorf("Error creating the AMQP request queue: %s", err)
	}
	err = consumeReponseMessages()
	if err != nil {
		return fmt.Errorf("Response queue consume error: %s", err)
	}
	err = consumeRequestMessages()
	if err != nil {
		return fmt.Errorf("Request queue consume error: %s", err)
	}

	return nil
}

// GetResponseQueueName doesn't need description.
func GetResponseQueueName() string {
	return responseQueueName
}

// CreateNewChannel is used to create new channels.
// This func is intended for use outside of this package, for example
// in the proxy package, it will create one channel per http request.
func CreateNewChannel() (*amqp.Channel, *Error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, createError("unexpected", "Unable to create new channel", map[string]string{"trace": err.Error()})
	}
	return ch, nil
}

// Declare the channel and queue we'll use for getting the response messages.
// Notice that this queue needs to be exclusive. This unique instance will be
// consuming from that queue. Plus, that queue will be destroyed when this
// instance gets disconnected.
func declareResponseChannelAndQueue() error {
	var err error
	responseChannel, err = conn.Channel()
	if err != nil {
		return err
	}
	responseQueueName = getResponseQueueName()
	_, err = responseChannel.QueueDeclare(
		responseQueueName, // Name
		true,              // Durable
		true,              // Delete when unused
		false,             // Exclusive
		false,             // No-wait
		nil,               // arguments
	)
	return err
}

func ensureRequestQueue() error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	_, err = ch.QueueDeclare(
		getRequestQueueName(), // Name
		true,  // Durable
		true,  // Delete when unused
		false, // Exclusive
		false, // No-wait
		nil,   // arguments
	)
	return err
}

func getRequestQueueName() string {
	return fmt.Sprintf("postman.req.%s", serviceName)
}

func getResponseQueueName() string {
	uniqid := uuid.NewV4()
	return fmt.Sprintf("postman.resp.%s", uniqid)
}

// Close the connection to the AMQP server.
func Close() {
	if responseChannel != nil {
		responseChannel.Close()
	}
	if conn != nil {
		conn.Close()
	}
}

// Consume messages on the response queue.
func consumeReponseMessages() error {
	msgs, err := responseChannel.Consume(
		responseQueueName, // Queue name
		"",                // Consumer
		true,              // Auto ack
		true,              // Exclusive
		false,             // No-local
		false,             // No-wait
		nil,               // args
	)
	if err != nil {
		return err
	}
	go func() {
		for d := range msgs {
			err := processMessageResponse(d.Body)
			if err != nil {
				log.Error(err)
			}
		}
	}()
	return nil
}

// Consume messages on the request queue.
// Note that this is a shared queue, a non exclusive queue.
func consumeRequestMessages() error {
	msgs, err := responseChannel.Consume(
		getRequestQueueName(), // Queue name
		"",    // Consumer
		false, // Auto ack
		false, // Exclusive
		false, // No-local
		false, // No-wait
		nil,   // args
	)
	if err != nil {
		return err
	}
	go func() {
		for d := range msgs {
			if err := processMessageRequest(d.Body); err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("Error processing request")
			}
			d.Ack(false)
		}
	}()
	return nil
}

func extractServiceNameFromQueueName(queueName string) string {
	parts := strings.Split(queueName, ".")
	return parts[len(parts)-1]
}

func queueExists(ch *amqp.Channel, queueName string) bool {
	_, err := ch.QueueInspect(queueName)
	if err == nil {
		return true
	}
	return false
}

func publishMessage(ch *amqp.Channel, message []byte, queueName string) *Error {
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
	if err == nil {
		return nil
	}
	return createError("unexpected", err.Error(), nil)
}

func createInvalidQueueNameError(queueName string) *Error {
	return createError(
		"queue_not_found",
		"The service name is invalid or there is no service instances available at the moment",
		map[string]string{
			"queue_name": queueName,
		},
	)
}

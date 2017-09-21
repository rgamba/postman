package async

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rgamba/postman/async/protobuf"
	"github.com/satori/go.uuid"
	"github.com/streadway/amqp"

	log "github.com/sirupsen/logrus"
)

var (
	// OnNewResponse execute this method each time a new
	// message response gets to the response queue.
	OnNewResponse func(protobuf.Response)
	// OnNewRequest will get executed each time a new requests
	// gets delivered to our instance.
	OnNewRequest func(protobuf.Request)
	// We'll use only one connection, this is the one.
	conn *amqp.Connection
	// These channels are only used for new message consuming.
	responseChannel *amqp.Channel
	requestChannel  *amqp.Channel
	mutex           = &sync.Mutex{}
	// The queue name we'll consume the response messages from.
	responseQueueName string
	// We'll store the service name here.
	serviceName string
	// Signal to notify if connection fails
	connCloseError = make(chan *amqp.Error)
)

func connectToServer(uri string) *amqp.Connection {
	for {
		con, err := amqp.Dial(uri)
		if err == nil {
			return con
		}
		log.WithFields(log.Fields{
			"error": err,
		}).Warnf("Connection error")
		log.Info("Trying to reconnect to AMQP server")
		time.Sleep(1 * time.Second)
	}
}

func serverConnector(uri string) {
	var amqpErr *amqp.Error

	for {
		amqpErr = <-connCloseError
		if amqpErr != nil {
			log.Infof("Connecting to %s", uri)

			conn = connectToServer(uri)
			connCloseError = make(chan *amqp.Error)
			conn.NotifyClose(connCloseError)

			// Response queue
			err := consumeReponseMessages()
			if err != nil {
				continue
			}
			// Request queue
			err = consumeRequestMessages()
			if err != nil {
				continue
			}
		}
	}
}

// Connect starts the connection to the AMQP server.
func Connect(uri string, service string) {
	serviceName = service
	connCloseError = make(chan *amqp.Error)
	go serverConnector(uri)
	connCloseError <- amqp.ErrClosed
}

// GetResponseQueueName doesn't need description.
func GetResponseQueueName() string {
	return responseQueueName
}

// GetServiceName doesn't need description.
func GetServiceName() string {
	return serviceName
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
		responseChannel.Cancel(responseQueueName, false)
		responseChannel.Close()
	}
	if requestChannel != nil {
		requestChannel.Close()
	}
	if conn != nil {
		conn.Close()
	}
}

// Consume messages on the response queue.
func consumeReponseMessages() error {
	err := declareResponseChannelAndQueue()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Errorf("Error creating the response queue")
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Errorf("Error creating channel for response")
		return err
	}

	msgs, err := ch.Consume(
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
	go func(ch *amqp.Channel) {
		defer ch.Close()
		for d := range msgs {
			err := processMessageResponse(d.Body)
			if err != nil {
				log.Error(err)
			}
		}
		go consumeReponseMessages()
		log.Warn("Stopped consuming response messages")
	}(ch)
	return nil
}

// Consume messages on the request queue.
// Note that this is a shared queue, a non exclusive queue.
func consumeRequestMessages() error {
	// Ensure there is a request queue declared.
	err := ensureRequestQueue()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Errorf("Error creating the request queue")
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Errorf("Error creating channel for request")
		return err
	}
	msgs, err := ch.Consume(
		getRequestQueueName(), // Queue name
		"",    // Consumer
		false, // Auto ack
		false, // Exclusive
		false, // No-local
		false, // No-wait
		nil,   // args
	)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatalf("Error creating request channel")
	}
	go func(ch *amqp.Channel) {
		defer ch.Close()
		for d := range msgs {
			if err := processMessageRequest(d.Body); err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("Error processing request")
			}
			d.Ack(false)
		}
		go consumeRequestMessages()
		log.Warn("Stopped consuming request messages")
	}(ch)
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

// GetServiceInstances returns the number of consumers of a given request
// queue on the AMQP server. In other words that means the number of instances
// available for that given service.
func GetServiceInstances(serviceName string) int {
	ch, err := conn.Channel()
	if err != nil {
		return 0
	}
	queueName := buildRequestQueueName(serviceName)
	queue, err := ch.QueueInspect(queueName)
	if err != nil {
		return 0
	}
	return queue.Consumers
}

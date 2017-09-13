package async

import (
	"fmt"
	"sync"

	"github.com/satori/go.uuid"
	"github.com/streadway/amqp"

	log "github.com/sirupsen/logrus"
)

// OnNewResponse execute this method each time a new
// message response gets to the response queue.
var OnNewResponse func([]byte)

// OnNewRequest will get executed each time a new requests
// gets delivered to our instance.
var OnNewRequest func([]byte)

var conn *amqp.Connection
var responseChannel *amqp.Channel
var requestChannel *amqp.Channel
var sendChannels = map[string]*amqp.Channel{}
var mutex = &sync.Mutex{}
var responseQueueName string
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

func GetResponseQueueName() string {
	return responseQueueName
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
			if OnNewResponse != nil {
				go OnNewResponse(d.Body)
			}
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
			if OnNewRequest != nil {
				go OnNewRequest(d.Body)
			}
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

func getOrCreateChannelForQueue(queueName string, checkQueueExists bool) (*amqp.Channel, error) {
	ch, found := getSendChannelCache(queueName)
	if found {
		if !queueExists(ch, queueName) && checkQueueExists {
			deleteSendChannelCache(queueName)
			return nil, fmt.Errorf("No queue declared for the service '%s'", queueName)
		}
		return ch, nil
	}
	// There is no channel in the map, we'll open a new channel and save
	// it on our sendChannel cache.
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	if !queueExists(ch, queueName) && checkQueueExists {
		return nil, fmt.Errorf("No queue declared for the service '%s'", queueName)
	}
	cacheSendChannel(queueName, ch)
	return ch, nil
}

func getSendChannelCache(queueName string) (*amqp.Channel, bool) {
	mutex.Lock()
	defer mutex.Unlock()
	ch, ok := sendChannels[queueName]
	return ch, ok
}

func deleteSendChannelCache(queueName string) {
	mutex.Lock()
	defer mutex.Unlock()
	delete(sendChannels, queueName)
}

func cacheSendChannel(queueName string, ch *amqp.Channel) {
	mutex.Lock()
	defer mutex.Unlock()
	sendChannels[queueName] = ch
}

func queueExists(ch *amqp.Channel, queueName string) bool {
	_, err := ch.QueueInspect(queueName)
	if err == nil {
		return true
	}
	return false
}

func sendMessageToQueue(message []byte, queueName string, checkQueueExists bool) error {
	ch, err := getOrCreateChannelForQueue(queueName, checkQueueExists)
	if err != nil {
		return err
	}
	err = publishMessage(ch, message, queueName)
	return err
}

func publishMessage(ch *amqp.Channel, message []byte, queueName string) error {
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

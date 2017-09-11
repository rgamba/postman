package async

import (
	"fmt"

	"github.com/satori/go.uuid"
	"github.com/streadway/amqp"
)

// OnNewMessage execute this method each time a new
// message gets to our response queue.
var OnNewMessage func([]byte)

var conn *amqp.Connection
var responseChannel *amqp.Channel
var sendChannels map[string]*amqp.Channel
var responseQueueName string

// Connect starts the connection to the AMQP server.
func Connect(uri string) error {
	var err error
	conn, err = amqp.Dial(uri)
	if err != nil {
		return fmt.Errorf("Unable to connect to the AMQP server: %s", err)
	}
	err = declareResponseChannelAndQueue()
	if err != nil {
		return fmt.Errorf("Error creating the AMQP response channel: %s", err)
	}
	err = consumeReponseMessages()
	if err != nil {
		return fmt.Errorf("Response queue consume error: %s", err)
	}
	return nil
}

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
		false,             // Delete when unused
		true,              // Exclusive
		false,             // No-wait
		nil,               // arguments
	)
	return err
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

func consumeReponseMessages() error {
	msgs, err := responseChannel.Consume(
		responseQueueName, // Queue name
		"",                // Consumer
		false,             // Auto ack
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
			if OnNewMessage != nil {
				OnNewMessage(d.Body)
			}
		}
	}()
	return nil
}

package stats

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// Outgoing request
	Outgoing = 1
	// Incoming request
	Incoming = 2
)

// Event is just a stat event.
// Will be only used internally on this package.
type Event struct {
	Value     int
	Timestamp int64
	Type      int // Outgoing or Incomming
	Metadata  interface{}
}

var serviceRequests = map[string][]Event{}
var mutex sync.RWMutex

// RecordRequest needs to be called each time we want to
// record a request being made to a service.
// service parameter must be the destination service.
func RecordRequest(service string, reqType int) {
	mutex.Lock()
	defer mutex.Unlock()
	event := Event{
		Value:     1,
		Timestamp: time.Now().Unix(),
		Type:      reqType,
	}
	serviceRequests[service] = append(serviceRequests[service], event)
}

// CountRequestsLastMinute will return the number of requests
// made to the service *service* in the last minute.
func CountRequestsLastMinute(service string, reqType int) (count int) {
	events := getServiceRequests(service)
	if events == nil {
		return 0
	}
	for _, event := range events {
		if reqType == event.Type && isLessThanOneMinuteOld(event) {
			count += event.Value
		}
	}
	return count
}

func GetRequestsLastMinutePerService(reqType int) map[string]int {
	result := map[string]int{}
	for serviceName, _ := range serviceRequests {
		result[serviceName] = CountRequestsLastMinute(serviceName, reqType)
	}
	return result
}

func isLessThanOneMinuteOld(event Event) bool {
	if event.Timestamp > (time.Now().Unix() - 60) {
		return true
	}
	return false
}

func getServiceRequests(service string) []Event {
	mutex.Lock()
	defer mutex.Unlock()
	events, ok := serviceRequests[service]
	if !ok {
		return nil
	}
	return events
}

// AutoPurgeOldEvents will be used as a means to
// periodically delete old events and prevent high memory
// utilization for the stats.
func AutoPurgeOldEvents() {
	go func() {
		time.Sleep(1 * time.Minute)
		log.Debug("Purging old events")
		purgeOldEvents()
	}()
}

func purgeOldEvents() {
	for service, events := range serviceRequests {
		for i, event := range events {
			if isOldEvent(event) {
				deleteEvent(service, i)
			}
		}
	}
}

func deleteEvent(service string, index int) {
	mutex.Lock()
	defer mutex.Unlock()
	serviceRequests[service] = append(serviceRequests[service][:index], serviceRequests[service][index+1:]...)
}

func isOldEvent(event Event) bool {
	var threshold int64
	threshold = 60 * 10 // 10 minutes.
	if event.Timestamp < (time.Now().Unix() - threshold) {
		return true
	}
	return false
}

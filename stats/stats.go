package stats

import (
	"sync"
	"time"
)

// Event is just a stat event.
// Will be only used internally on this package.
type Event struct {
	Value     int
	Timestamp int64
	Metadata  interface{}
}

var serviceRequests = map[string][]Event{}
var mutex sync.RWMutex

// RecordRequest needs to be called each time we want to
// record a request being made to a service.
// service parameter must be the destination service.
func RecordRequest(service string) {
	mutex.Lock()
	defer mutex.Unlock()
	event := Event{
		Value:     1,
		Timestamp: time.Now().Unix(),
	}
	serviceRequests[service] = append(serviceRequests[service], event)
}

// CountRequestsLastMinute will return the number of requests
// made to the service *service* in the last minute.
func CountRequestsLastMinute(service string) (count int) {
	events := getServiceRequests(service)
	if events == nil {
		return 0
	}
	for _, event := range events {
		if isLessThanOneMinuteOld(event) {
			count += event.Value
		}
	}
	return count
}

func isLessThanOneMinuteOld(event Event) bool {
	if event.Timestamp < (time.Now().Unix() - 60) {
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

// PurgeOldEvents will be used as a means to
// periodically delete old events and prevent high memory
// utilization for the stats.
func PurgeOldEvents() {
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
	threshold = 60 * 60 // 1 hour
	if event.Timestamp < (time.Now().Unix() - threshold) {
		return true
	}
	return false
}

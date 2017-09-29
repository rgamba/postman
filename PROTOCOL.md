# Services

A service must be identified by a `service_id`. Many instances can share the same
`service_id`, in which case, we assume all those instances strictly share the same
application and version.

The `service_id`s should be publicly available in a repository or elsewhere, so all
services know all  other services IDs.

## Service identifier

`service_id` must evaluate to the following regex: `[a-zA-z0-9\-_\.]` (only letters, 
numbers, underscores, colons and hyphens).

# Queues

Each service will work with 2 queues.

The first queue will be a shared queue and will work as a task queue.

This queue will handle all the incomming requests for a given service
and will load balance the requests among all service instances connnected
and listening to that queue.

The name of the queue will be:

```
postman.request.<service_id>
```

Where `service_id` must be replaced by the service unique id name.

The second queue will be a unique queue per service instance. It will **not**
be a shared queue and it will store responses to the requests.

The queue name will be:

```
postman.response.<uuid4>
```

Where `uuid4` must be replaced by a [UUIDv4](https://tools.ietf.org/html/rfc4122) string.


# Messages

## New request message

Whenever a new request needs to be issued, a new message needs to be sent to the
service queue (`postman.request.<service_id>`).

Postman messages use Protocol Buffer for data serialization and as the format of our messages.

The definition (`.proto`) for the request message is as follows:

```protobuf
message Request {
    string id = 1;
    string method = 2;
    string endpoint = 3;
    repeated string headers = 4;
    string response_queue = 5;
    string body = 6;
    string service = 7; // Requesting service
}
```

### Versioning

A message version must be included in the AMQP message header `version`. The current version is `1`.

It is important to include the message version, that way in case the client processing the message is incompatible
with the message version, the message will be put back into the queue so other instance can process it.

## Response message

When the request has been processed by any of the instances, a response message must be created and sent
through the `response_queue` queue **except** when `response_queue` is null, in which case, the requester
doesn't expect a response and it must not be sent.

The definition (`.proto`) for the response message is as follows:

```protobuf
message Response {
    string request_id = 1; // Same as Request.id
    int32 status_code = 2;
    repeated string headers = 3;
    string body = 4;
}
```

# Exceptions

In case there was an error processing the request or interpreting the request message, a message with an empty
payload must be sent back to the `request_queue` (if provided), with the `error` AMQP header with the appropriate
error code.

## Error codes

Code            | Description       | Expected action
--              | --                | --
invalid_version | The client is unable to process the message version, is incompatible. | The message must be ack. Then a clonned message with  the AMQP header `retry` value incremented by 1 (1 if header didn't existed).
invalid_format  | Error while trying to decode message. | None   
no_available_instances | When there are no available instances at the moment to process the request. | Assume there is a service outage at the moment and cache the unavailable service momentarily for 1 min.

# AMQP Headers

The AMQP message can sometimes have one or more AMQP headers. The client must inspect the headers and act acoardingly:

Header | Possible values | Expected action
-- | -- | --
error | One of the error codes | Inspect and take action based on the error code.
unhealthy_count | [0-3] | If `fwd_host` is healthy, ignore. If it's unhealthy, then: IF the value >= 3 THEN send `no_available_instances` error message. ELSE, increment the value by 1 and put the message back in the request queue.

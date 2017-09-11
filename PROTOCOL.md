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
    required string id = 1;
    required string endpoint = 2;
    repeated string headers = 3;
    required string response_queue = 4;
    optional string body = 5;
}
```

## Response message

When the request has been processed by any of the instances, a response message will be created and sent
through the `response_queue` queue.

The definition (`.proto`) for the response message is as follows:

```protobuf
message Response {
    required string request_id = 1; // Same as Request.id
    required int32 status_code = 2;
    repeated string headers = 3;
    optional string body = 4;
}
```
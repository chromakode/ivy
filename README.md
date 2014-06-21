![ivy](https://raw.githubusercontent.com/chromakode/ivy/master/art/logo.png)

Ivy is a minimalistic hierarchical pubsub server with persistent event logs. It's designed for real time shared spaces such as chat rooms or collaborative creative tools. While a standalone binary is provided, Ivy is more oriented towards embedding in a larger server to provide the core realtime event system.

Ivy is written in Go, and uses [LevelDB](https://code.google.com/p/leveldb/) for persistence. It provides Websocket and HTTP views of event streams.

**Note:** Ivy is not stable or production ready yet, and lacks a thorough test suite. The protocol will change without warning in the future. It is suitable only for experimental or demo use. It also doesn't scale. You really shouldn't use this unless you like hacking on Go programs. It may eat your data, or more likely your time.


---


## structure

In Ivy, *clients* subscribe to *channels*, which exist in a hierarchical tree starting at the root, `/`. Clients send *events* to channels, which bubble up to parent channels, until they reach the root. Ivy is agnostic about event format; event data is treated as a blob of bytes. In practice, JSON is usually most convenient.

Every event in Ivy is persisted to the `logs`, which are accessible via HTTP at:

    http(s)://hostname/log/path/to/channel

Real time events are sent over the WebSocket interface at:

    ws(s)://hostname/ws


## protocol

Ivy uses a simple ASCII protocol that is intended to be mildly human-readable. Commands and events are newline-terminated. In event data, newlines are escaped to `%0A`, and '%' characters to `%25`.

### client commands


#### callbacks / acks

To receive an acknowledge response from the server, prefix a command with:

    #ackkey#

The *ack key* can be any data that is convenient for the client to associate the response with its initial request.


#### subscribe / unsubscribe

    +/path/to/channel
    -/path/to/channel

Subscribe / unsubscribe from a channel. Subsequent events will be sent to the client.


#### sending events

    :/path/to/channel:data

Send an event to a channel with *data*.


#### server timestamp

    @

Request the server timestamp in floating point format (seconds.nanoseconds). Requires an *ack key*.


### receiving events

Events sent to the client (and in logs) take the following format:

    @seconds.nanoseconds:/path/to/channel:data


## client libs

Ivy has a basic [JavaScript client library](https://github.com/chromakode/ivy-client.js).


## future plans

 * authentication
 * channel join/part events
 * channel subscriber rosters
 * "confirmed" events sent with the timestamp of the last received event (for handling collisions)
 * offset log fetching with infinite cache headers (for use behind a pull CDN)
 * multiple host scaling

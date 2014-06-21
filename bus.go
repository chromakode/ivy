package ivy

import (
	"fmt"
	"strings"
	"time"
)

type connectionMap map[*connection]bool

type bus struct {
	tree    map[string]connectionMap
	control chan actionHandler
	logger  *logger
}

func newBus(logger *logger) *bus {
	return &bus{
		tree:    make(map[string]connectionMap),
		control: make(chan actionHandler),
		logger:  logger,
	}
}

func (bus *bus) run() {
	for {
		action := <-bus.control
		action.handle(bus)
	}
}

type actionHandler interface {
	handle(*bus)
}

type subscribeAction struct {
	path string
	conn *connection
}

func (sub subscribeAction) handle(bus *bus) {
	if bus.tree[sub.path] == nil {
		bus.tree[sub.path] = make(connectionMap)
	}
	bus.tree[sub.path][sub.conn] = true
}

type unsubscribeAction subscribeAction

func (unsub unsubscribeAction) handle(bus *bus) {
	delete(bus.tree[unsub.path], unsub.conn)
	if len(bus.tree[unsub.path]) == 0 {
		delete(bus.tree, unsub.path)
	}
}

type eventAction struct {
	timestamp time.Time
	path      string
	conn      *connection
	data      []byte
}

func formatEvent(ev eventAction) string {
	// simple slow escaping
	data := strings.Replace(string(ev.data), "%", "%25", -1)
	data = strings.Replace(data, "\n", "%0A", -1)
	return fmt.Sprintf("%s:%s:%s", timeString(ev.timestamp), ev.path, data)
}

func (ev eventAction) handle(bus *bus) {
	eventLine := formatEvent(ev)

	bus.logger.log <- logEntry{
		timestamp: ev.timestamp,
		path:      ev.path,
		line:      eventLine,
	}

	broadcast := func(path string) {
		for c := range bus.tree[path] {
			if c == ev.conn {
				continue
			}

			select {
			case c.send <- []byte(eventLine):
			default:
				close(c.send)
				unsubscribeAction{path, c}.handle(bus)
			}
		}
	}

	// bubble the event up to each parent path
	path := ev.path
	for idx := len(path); idx != -1; idx = strings.LastIndex(path, "/") {
		path = path[:idx]
		broadcast(path)
	}
}

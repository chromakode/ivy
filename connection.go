package ivy

import (
	"bytes"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 1024 * 64
)

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"ivy1"},
}

type connection struct {
	bus           *bus
	ws            *websocket.Conn
	subscriptions map[string]bool
	send          chan []byte
}

var validPath = regexp.MustCompile(`^/[0-9a-z-./]+$`)

func normalizePath(path string) string {
	path = strings.TrimSpace(path)

	// trim trailing /
	if path[len(path)-1] == '/' {
		path = path[:len(path)-2]
	}

	// check for invalid characters
	if !validPath.MatchString(path) {
		return ""
	}

	return path
}

func dropLast(bytes []byte) []byte {
	return bytes[:len(bytes)-1]
}

// readPump pumps messages from the websocket connection to the hub.
func (c *connection) readPump() {
	defer c.cleanup()
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	var (
		messageType int
		messageData []byte
		err         error
	)

ReadLoop:
	for {
		messageType, messageData, err = c.ws.ReadMessage()

		if err == io.EOF {
			return
		}

		if err != nil {
			log.Printf("socket error: %v", err)
			return
		}

		if messageType != websocket.TextMessage {
			log.Printf("socket message of wrong type: %v", messageType)
			return
		}

		message := bytes.NewBuffer(messageData)
		cmd, err := message.ReadByte()
		if err != nil {
			break ReadLoop
		}

		ackKey := []byte{}
		if cmd == '#' {
			ackKey, err = message.ReadBytes('#')
			if err != nil {
				break ReadLoop
			}
			ackKey = ackKey[:len(ackKey)-1]

			cmd, err = message.ReadByte()
			if err != nil {
				break ReadLoop
			}
		}

		timestamp := time.Now().UTC()

		switch cmd {
		case '+', '-':
			path := normalizePath(message.String())
			if path == "" {
				break ReadLoop
			}

			if cmd == '+' {
				c.subscribe(path)
			} else {
				c.unsubscribe(path)
			}
		case ':':
			pathBytes, err := message.ReadBytes(':')
			if err != nil {
				break ReadLoop
			}

			path := normalizePath(string(dropLast(pathBytes)))
			if path == "" {
				break ReadLoop
			}

			c.bus.control <- eventAction{timestamp, path, c, message.Bytes()}
		case '@':
			// FIXME: investigate GC pauses (thanks intortus!)
			resp := new(bytes.Buffer)
			fmt.Fprintf(resp, "#%s#%s", ackKey, timeString(timestamp))
			c.send <- resp.Bytes()
		default:
			// >:(
			break ReadLoop
		}
	}

	log.Printf("invalid msg: %q", messageData)
}

func (c *connection) subscribe(path string) {
	c.bus.control <- subscribeAction{path, c}
	c.subscriptions[path] = true
}

func (c *connection) unsubscribe(path string) {
	c.bus.control <- unsubscribeAction{path, c}
	delete(c.subscriptions, path)
}

func (c *connection) cleanup() {
	for path := range c.subscriptions {
		c.unsubscribe(path)
	}
	c.ws.Close()
}

func (c *connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

func (c *connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.cleanup()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func connectWs(b *bus, w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}
	c := &connection{
		bus:           b,
		ws:            ws,
		subscriptions: make(map[string]bool),
		send:          make(chan []byte, 256),
	}
	go c.writePump()
	c.readPump()
}

package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CarboxyDev/go-chat/internal/constants"
	"github.com/fatih/color"
)

type MessageType string

const (
	MessageJoin MessageType = "join"
	MessageLeave MessageType = "leave"
	MessageChat MessageType = "chat"
	MessageSystem MessageType = "system"
	MessageNick MessageType = "nick"

)

type Message struct {
	Type MessageType `json:"type"`
	FromID string `json:"fromId,omitempty"`
	FromName string `json:"fromName,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Body string `json:"body,omitempty"`

}

type Broadcast struct {
	from *Client;
	data []byte;
}

// Hub or Room state, 1 goroutine
type Hub struct {
	register chan *Client
	unregister chan *Client
	broadcast chan Broadcast
	clients map[*Client]bool
}


// Client Represents 1 TCP Connection 
type Client struct {
	conn net.Conn
	send chan []byte;
	id string;
	name string;
}


func CreateHub() *Hub {
	return &Hub {
		register: make(chan *Client),
		unregister: make(chan *Client),
		broadcast: make(chan Broadcast, 256),
		clients: make(map[*Client]bool),
	}
}

func (hub *Hub) Run() {
	for {
		select {
		case c := <-hub.register:
			hub.clients[c] = true;
		case c := <-hub.unregister:
			if hub.clients[c] {
				// Cleanup channels and entry
				delete(hub.clients, c);
				close(c.send);
				c.conn.Close();
			}
			
		case msg := <-hub.broadcast:
			for c := range hub.clients {
				select {
				case c.send <- msg.data:
				default: 
					// Slow client
				}
			}
		}
	}
}



func main() {
    const PORT = "8080"
    server, err := net.Listen("tcp", ":"+PORT)
    if err != nil { panic(err) }
    defer server.Close()

    color.Green("[+] Server started on %s", PORT)

    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

    hub := CreateHub()
    go hub.Run()

    go func() {
        <-sigs
        color.Red("[!] Closing the server")
        _ = server.Close()
        // Optionally: close all clients by asking hub to iterate and close.
        os.Exit(0)
    }()

    for {
        conn, err := server.Accept()
        if err != nil {
            if ne, ok := err.(net.Error); ok && ne.Temporary() {
                color.Yellow("[~] Temporary accept error: %v", err)
                continue
            }
            color.Red("[x] Accept error: %v", err)
            return
        }
        go connectionHandler(hub, conn)
    }
}


func connectionHandler(hub *Hub, conn net.Conn) {
    c := &Client{
        conn: conn,
        send: make(chan []byte, 256),
        id: conn.RemoteAddr().String(),
    }

    hub.register <- c;

    go writePump(c)     
    readPump(hub, c)    
}


func writePump(c *Client) {
	bufWriter := bufio.NewWriter(c.conn)
    for msg := range c.send {
        if err := writeFrame(bufWriter, msg); err != nil { break }
        if err := bufWriter.Flush(); err != nil { break }
    }
	c.conn.Close()

}

func readPump(hub *Hub, c *Client) {
    bufReader := bufio.NewReader(c.conn)

    for {
        frame, err := readFrame(bufReader, constants.MaxFrameSize)
        if err != nil { 
            hub.unregister <- c
            return
        }

		// Frame = JSON payload from the client 
		hub.broadcast <- Broadcast{from: c, data: frame}
    }
}

// writeFrame writes len(payload) as 4-byte big-endian then payload.
func writeFrame(w io.Writer, payload []byte) error {
    var hdr [4]byte
    binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
    if _, err := w.Write(hdr[:]); err != nil { return err }
    _, err := w.Write(payload)
    return err
}

// readFrame reads a single length-prefixed frame with a max cap.
func readFrame(r io.Reader, max int) ([]byte, error) {
    var hdr [4]byte
    if _, err := io.ReadFull(r, hdr[:]); err != nil { return nil, err }
    n := int(binary.BigEndian.Uint32(hdr[:]))
    if n < 0 || n > max { return nil, fmt.Errorf("frame too large: %d", n) }
    buf := make([]byte, n)
    if _, err := io.ReadFull(r, buf); err != nil { return nil, err }
    return buf, nil
}

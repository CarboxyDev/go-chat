package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

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
    for msg := range c.send {
        if _, err := c.conn.Write(msg); err != nil {
            return
        }
    }
}

func readPump(hub *Hub, c *Client) {
    buf := make([]byte, 4096)
    for {
        n, err := c.conn.Read(buf)
        if err != nil { // EOF or network error
            hub.unregister <- c
            return
        }
        if n > 0 {
            // copy because buf will be reused
            payload := make([]byte, n)
            copy(payload, buf[:n])
            hub.broadcast <- Broadcast{from: c, data: payload}
        }
    }
}
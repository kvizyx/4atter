package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

const (
	host        = "localhost"
	defaultPort = "7777"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleConnection(conn *websocket.Conn, messages chan Message) {
	client := NewClient(conn)

	defer func() {
		messages <- Message{
			Type:   ClientDisconnected,
			Sender: client,
		}

		conn.Close()
	}()

	messages <- Message{
		Type:   ClientConnected,
		Sender: client,
	}

	for {
		_, data, err := client.conn.ReadMessage()
		if err != nil {
			return
		}

		messages <- Message{
			Type:   NewMessage,
			Text:   string(data),
			Sender: client,
		}
	}
}

func handleUpgrade(messages chan Message) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("failed to upgrade connection: %s\n", err)
			return
		}

		go handleConnection(conn, messages)
	}
}

func main() {
	port := flag.String("port", defaultPort, "port")
	flag.Parse()

	addr := fmt.Sprintf("%s:%s", host, *port)

	room := NewRoom(nil)
	go room.Serve()

	http.HandleFunc("/", handleUpgrade(room.Messages))

	log.Printf("chat server started on ws://%s\n", addr)

	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("failed to listen http on %s: %s", addr, err)
	}
}

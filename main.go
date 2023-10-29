package main

import (
	"flag"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
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
	senderID, _ := uuid.NewV4()
	sender := &Client{
		ID:   senderID,
		Conn: conn,
	}

	defer func() {
		messages <- Message{
			Type:   ClientDisconnected,
			Sender: sender,
		}

		conn.Close()
	}()

	messages <- Message{
		Type:   ClientConnected,
		Sender: sender,
	}

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return
			}

			log.Printf(
				"Failed to read message of type %d from %s: %s\n",
				msgType, conn.RemoteAddr().String(), err,
			)
			return
		}

		messages <- Message{
			Type:   NewMessage,
			Text:   string(data),
			Sender: sender,
		}
	}
}

func handleUpgrade(messages chan Message) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf(
				"Failed to upgrade connection from %s: %s\n",
				conn.RemoteAddr().String(), err,
			)
			return
		}

		go handleConnection(conn, messages)
	}
}

func main() {
	port := flag.String("port", defaultPort, "port")
	flag.Parse()

	addr := fmt.Sprintf("%s:%s", host, *port)

	room := NewRoom()
	go room.Serve()

	http.HandleFunc("/", handleUpgrade(room.Messages))

	log.Printf("chat server started on ws://%s\n", addr)

	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("Failed to listen http on %s: %s", addr, err)
	}
}

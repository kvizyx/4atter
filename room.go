package main

import (
	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"log"
	"time"
)

type MessageType int

const (
	ClientConnected MessageType = iota
	ClientDisconnected
	NewMessage
)

// all values represent seconds
const (
	MessageLimit = 1.0
	CooldownTime = 10.0
)

type Client struct {
	ID   uuid.UUID
	Conn *websocket.Conn

	LastMessageTime time.Time
	LastMessageText string

	CooldownMessage string
	CooldownStart   time.Time
}

type Message struct {
	Type   MessageType
	Text   string
	Sender *Client
}

type Room struct {
	Clients  map[uuid.UUID]*Client
	Messages chan Message
}

func NewRoom() *Room {
	return &Room{
		Clients:  make(map[uuid.UUID]*Client),
		Messages: make(chan Message),
	}
}

func (r *Room) Serve() {
	for {
		msg := <-r.Messages

		switch msg.Type {
		case ClientConnected:
			r.handleConnect(msg)
		case ClientDisconnected:
			r.handleDisconnect(msg)
		case NewMessage:
			r.handleNewMessage(msg)
		}
	}
}

func (r *Room) handleConnect(msg Message) {
	msg.Sender.LastMessageTime = time.Now()
	r.Clients[msg.Sender.ID] = msg.Sender

	log.Printf("Client %s connected\n", msg.Sender.ID)
}

func (r *Room) handleDisconnect(msg Message) {
	delete(r.Clients, msg.Sender.ID)
	log.Printf("Client %s disconnected\n", msg.Sender.ID)
}

func (r *Room) handleNewMessage(msg Message) {
	if len(msg.Text) == 0 {
		return
	}

	if time.Now().Sub(msg.Sender.LastMessageTime).Seconds() < MessageLimit {
		return
	}

	if msg.Text == msg.Sender.LastMessageText {
		if msg.Sender.CooldownMessage == "" {
			msg.Sender.CooldownMessage = msg.Text
			msg.Sender.CooldownStart = time.Now()
			return
		}

		if time.Now().Sub(msg.Sender.CooldownStart).Seconds() < CooldownTime {
			return
		}

		msg.Sender.CooldownMessage = ""
	}

	msg.Sender.LastMessageTime = time.Now()
	msg.Sender.LastMessageText = msg.Text

	log.Printf(
		"Broadcasting message from %s: %s\n",
		msg.Sender.ID, msg.Text,
	)

	for _, client := range r.Clients {
		if client.ID == msg.Sender.ID {
			continue
		}

		err := client.Conn.WriteMessage(websocket.TextMessage, []byte(msg.Text))
		if err != nil {
			log.Printf(
				"Failed to send message to %s: %s\n",
				client.ID, err,
			)
			client.Conn.Close()
		}
	}
}

package main

import (
	"fmt"
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

type Message struct {
	Type   MessageType
	Text   string
	Sender *Client
}

type RoomSettings struct {
	MessageLimit        float64
	CooldownTime        float64
	CooldownHitsLimit   int
	CooldownHitsBanTime time.Duration
	MaxConnection       int
}

type Room struct {
	Clients       map[uuid.UUID]*Client
	BannedClients map[ClientIP]time.Time
	Messages      chan Message

	Settings *RoomSettings
}

func NewRoom(settings *RoomSettings) *Room {
	return &Room{
		Clients:       make(map[uuid.UUID]*Client),
		BannedClients: make(map[ClientIP]time.Time),
		Messages:      make(chan Message),
		Settings:      settings,
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
	if len(r.Clients) == r.Settings.MaxConnection {
		msg.Sender.Conn.WriteMessage(websocket.TextMessage, []byte("Room is full"))
		msg.Sender.Conn.Close()
	}

	bannedUntil, isBanned := r.BannedClients[msg.Sender.IP()]
	if isBanned {
		secondsLeft := bannedUntil.Sub(time.Now()).Seconds()

		if secondsLeft <= 0 {
			delete(r.BannedClients, msg.Sender.IP())
		} else {
			message := fmt.Sprintf(
				"You are banned from this room. %d seconds left",
				int(secondsLeft),
			)
			msg.Sender.Conn.WriteMessage(websocket.TextMessage, []byte(message))
			msg.Sender.Conn.Close()
			return
		}
	}

	msg.Sender.LastMessageTime = time.Now()
	r.Clients[msg.Sender.ID] = msg.Sender

	log.Printf("Client %s connected\n", msg.Sender.ID)
}

func (r *Room) handleDisconnect(msg Message) {
	delete(r.Clients, msg.Sender.ID)
	log.Printf("Client %s disconnected\n", msg.Sender.ID)
}

func (r *Room) handleNewMessage(msg Message) {
	sender := msg.Sender

	if len(msg.Text) == 0 {
		return
	}

	if time.Now().Sub(sender.LastMessageTime).Seconds() < r.Settings.MessageLimit {
		return
	}

	if msg.Text == sender.LastMessageText {
		if sender.CooldownMessage == "" {
			sender.CooldownMessage = msg.Text
			sender.CooldownStart = time.Now()
			return
		}

		if time.Now().Sub(sender.CooldownStart).Seconds() < r.Settings.CooldownTime {
			sender.CooldownHits += 1

			if sender.CooldownHits >= r.Settings.CooldownHitsLimit {
				r.banClient(sender, r.Settings.CooldownHitsBanTime, "spamming")
			}

			return
		}

		sender.CooldownMessage = ""
	}

	sender.LastMessageTime = time.Now()
	sender.LastMessageText = msg.Text

	log.Printf(
		"Broadcasting message from %s: %s\n",
		sender.ID, msg.Text,
	)

	for _, client := range r.Clients {
		if client.ID == sender.ID {
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

func (r *Room) banClient(
	client *Client,
	duration time.Duration,
	reason string,
) {
	r.BannedClients[client.IP()] = time.Now().Add(duration)

	log.Printf(
		"Client %s have been banned. Reason: %s, Duration: %s\n",
		client.ID, reason, duration,
	)

	banMessage := fmt.Sprintf(
		"You have been banned. Reason: %s, Duration: %s",
		reason, duration,
	)

	_ = client.Conn.WriteMessage(
		websocket.TextMessage,
		[]byte(banMessage),
	)

	client.Conn.Close()
}

package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofrs/uuid"
)

type MessageType int

var defaultSettings = &RoomSettings{
	MessageLimit:        1.0,
	CooldownTime:        10.0,
	CooldownHitsLimit:   10,
	CooldownHitsBanTime: 1 * time.Minute,
	MaxClients:          50,
	DefaultBanReason:    "unknown",
}

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
	MaxClients          int
	DefaultBanReason    string
}

type Room struct {
	Clients       map[uuid.UUID]*Client
	BannedClients map[ClientIP]time.Time
	Messages      chan Message
	Settings      *RoomSettings

	mu *sync.Mutex
}

func NewRoom(settings *RoomSettings) *Room {
	if settings == nil {
		settings = defaultSettings
	}

	return &Room{
		Clients:       make(map[uuid.UUID]*Client),
		BannedClients: make(map[ClientIP]time.Time),
		Messages:      make(chan Message),
		Settings:      settings,
		mu:            &sync.Mutex{},
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
	if len(r.Clients) == r.Settings.MaxClients {
		msg.Sender.WriteClose("Room is full, try to connect later")
		return
	}

	banEnd, isBanned := r.BannedClients[msg.Sender.IP()]
	if isBanned {
		secondsLeft := time.Until(banEnd).Seconds()

		if secondsLeft <= 0 {
			r.UnbanClient(msg.Sender.IP())
		} else {
			message := fmt.Sprintf(
				"You are banned from this room. %d seconds left",
				int(secondsLeft),
			)

			msg.Sender.WriteClose(message)
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

	if time.Since(sender.LastMessageTime).Seconds() < r.Settings.MessageLimit {
		return
	}

	if msg.Text == sender.LastMessageText {
		if !sender.HasCooldown {
			sender.HasCooldown = true
			sender.CooldownMessage = msg.Text
			sender.CooldownStart = time.Now()
			return
		}

		if time.Since(sender.CooldownStart).Seconds() < r.Settings.CooldownTime {
			sender.CooldownHits += 1

			if sender.CooldownHits >= r.Settings.CooldownHitsLimit {
				r.BanClient(
					sender,
					r.Settings.CooldownHitsBanTime,
					"spamming during cooldown",
				)
			}
			return
		}

		sender.HasCooldown = false
	}

	sender.LastMessageTime = time.Now()
	sender.LastMessageText = msg.Text

	log.Printf("Broadcasting message from %s\n", sender.ID)

	for _, client := range r.Clients {
		if client.ID == sender.ID {
			continue
		}

		client.MustWrite(msg.Text)
	}
}

func (r *Room) BanClient(
	client *Client,
	duration time.Duration,
	reason string,
) {
	if len(reason) == 0 {
		reason = r.Settings.DefaultBanReason
	}

	r.BannedClients[client.IP()] = time.Now().Add(duration)

	log.Printf(
		"Client %s have been banned. Reason: %s, Duration: %s\n",
		client.ID, reason, duration,
	)

	banMessage := fmt.Sprintf(
		"You have been banned from this room. Reason: %s, Duration: %s",
		reason, duration,
	)

	client.WriteClose(banMessage)
}

func (r *Room) UnbanClient(clientIP ClientIP) {
	delete(r.BannedClients, clientIP)
}

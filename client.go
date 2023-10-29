package main

import (
	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"net"
	"time"
)

type ClientIP string

type Client struct {
	ID   uuid.UUID
	Conn *websocket.Conn

	LastMessageTime time.Time
	LastMessageText string

	CooldownMessage string
	CooldownStart   time.Time
	CooldownHits    int
}

// IP return clean client ip address (without port)
func (c *Client) IP() ClientIP {
	clientAddr := c.Conn.RemoteAddr().(*net.TCPAddr)
	return ClientIP(clientAddr.IP.String())
}

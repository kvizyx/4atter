package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
)

const (
	connWriteDeadline  = 2 * time.Second
	connReadDeadline   = 2 * time.Second
	connMaxMessageSize = 512
)

var (
	ErrWriteConn = errors.New("failed to write to client's connection")
	ErrCloseConn = errors.New("failed to close client's connection")
	ErrReadConn  = errors.New("failed to read from client's connection")
)

type ClientIP string

type Client struct {
	ID   uuid.UUID
	conn *websocket.Conn

	LastMessageTime time.Time
	LastMessageText string

	HasCooldown     bool
	CooldownMessage string
	CooldownStart   time.Time
	CooldownHits    int

	mu *sync.Mutex
}

func NewClient(conn *websocket.Conn) *Client {
	clientID, _ := uuid.NewV4()

	return &Client{
		ID:   clientID,
		conn: conn,
		mu:   &sync.Mutex{},
	}
}

func (c *Client) Write(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	wrDeadline := time.Now().Add(connWriteDeadline)
	c.conn.SetWriteDeadline(wrDeadline)

	err := c.conn.WriteMessage(websocket.TextMessage, []byte(text))
	if err != nil {
		return fmt.Errorf("%s: %s", ErrWriteConn, err)
	}

	return nil
}

func (c *Client) WriteClose(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	wrDeadline := time.Now().Add(connWriteDeadline)
	c.conn.SetWriteDeadline(wrDeadline)

	message := websocket.FormatCloseMessage(
		websocket.CloseNormalClosure,
		text,
	)

	err := c.conn.WriteMessage(websocket.CloseMessage, message)
	if err != nil {
		return fmt.Errorf("%s: %s", ErrWriteConn, err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("%s: %s", ErrCloseConn, err)
	}

	return nil
}

func (c *Client) MustWrite(text string) {
	if err := c.Write(text); err != nil {
		log.Printf("%s, closing connection...: %s", ErrWriteConn, err)

		// FIXME: should i panic there?
		if err := c.conn.Close(); err != nil {
			log.Printf("%s: %s", ErrCloseConn, err)
		}
	}
}

// IP returns clean client ip without port
func (c *Client) IP() ClientIP {
	clientAddr := c.conn.RemoteAddr().(*net.TCPAddr)
	return ClientIP(clientAddr.IP.String())
}

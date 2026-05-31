package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	game   *Game
	player *Player
}

// ClientMessage represents any message from the client.
type ClientMessage struct {
	Type string  `json:"type"`
	Name string  `json:"name,omitempty"`
	DX   float64 `json:"dx,omitempty"`
	DY   float64 `json:"dy,omitempty"`
	X    float64 `json:"x,omitempty"`
	Y    float64 `json:"y,omitempty"`
	Text string  `json:"text,omitempty"`
}

func (c *Client) readPump() {
	defer func() {
		c.game.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read error: %v", err)
			}
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "join":
			if c.player == nil && msg.Name != "" {
				name := msg.Name
				if len(name) > 20 {
					name = name[:20]
				}
				c.game.join <- &JoinRequest{client: c, name: name}
			}
		case "input":
			if c.player != nil {
				c.player.SetDirection(msg.DX, msg.DY)
			}
		case "target":
			if c.player != nil {
				c.player.SetTarget(msg.X, msg.Y)
			}
		case "chat":
			if c.player != nil && msg.Text != "" {
				text := msg.Text
				if len(text) > 200 {
					text = text[:200]
				}
				c.game.chat <- &ChatRequest{
					sender: c.player.Name,
					text:   text,
				}
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

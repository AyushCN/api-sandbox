package api

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for sandbox
	},
}

// WsClient represents a single websocket connection
type WsClient struct {
	ID      string
	Conn    *websocket.Conn
	Send    chan interface{}
	EnvID   string
	UserID  string
}

// Hub maintains active clients and broadcasts messages
type Hub struct {
	clients    map[*WsClient]bool
	broadcast  chan BroadcastMessage
	register   chan *WsClient
	unregister chan *WsClient
	mu         sync.Mutex
}

type BroadcastMessage struct {
	EnvID string
	Data  interface{}
}

var WSHub = &Hub{
	broadcast:  make(chan BroadcastMessage),
	register:   make(chan *WsClient),
	unregister: make(chan *WsClient),
	clients:    make(map[*WsClient]bool),
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				// Broadcast to all clients connected to this environment
				if client.EnvID == message.EnvID {
					select {
					case client.Send <- message.Data:
					default:
						close(client.Send)
						delete(h.clients, client)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// ServeWS handles websocket requests from the peer
func ServeWS(c *gin.Context) {
	envID := c.Param("id")
	userID, exists := c.Get("userId")
	if !exists {
		userID = "anonymous"
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("websocket upgrade error:", err)
		return
	}

	client := &WsClient{
		ID:     conn.RemoteAddr().String(),
		Conn:   conn,
		Send:   make(chan interface{}, 256),
		EnvID:  envID,
		UserID: userID.(string),
	}
	WSHub.register <- client

	// Start pump goroutines
	go client.writePump()
	go client.readPump()
}

func (c *WsClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteJSON(message)
		case <-ticker.C:
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *WsClient) readPump() {
	defer func() {
		WSHub.unregister <- c
		c.Conn.Close()
	}()
	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func BroadcastToProjectMembers(envID string, data map[string]interface{}) {
	WSHub.broadcast <- BroadcastMessage{
		EnvID: envID,
		Data:  data,
	}
}

func GetCurrentUserName(userID string) string {
	var user models.User
	if err := db.DB.First(&user, "id = ?", userID).Error; err != nil {
		return "Unknown User"
	}
	if user.Username != "" {
		return user.Username
	}
	return user.Email
}

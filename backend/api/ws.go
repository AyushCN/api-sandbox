package api

import (
	"log"
	"net/http"
	"os"
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
		origin := r.Header.Get("Origin")
		frontendUrl := os.Getenv("FRONTEND_URL")
		if frontendUrl == "" {
			frontendUrl = "http://localhost:3000"
		}

		// Ensure origin matches exactly, avoiding substring matches like 'http://localhost:3000.malicious.com'
		return origin == frontendUrl
	},
}

// WsClient represents a single websocket connection
type WsClient struct {
	ID     string
	Conn   *websocket.Conn
	Send   chan interface{}
	EnvID  string
	UserID string
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
	Type      string
	EnvID     string
	UserID    string
	UserName  string
	Data      interface{}
	Timestamp time.Time
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
					case client.Send <- message:
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

	// Check if user has access to this environment's project
	var env models.Environment
	if err := db.DB.First(&env, "id = ?", envID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	var hasAccess bool
	if env.UserID == userID {
		hasAccess = true
	} else if env.ProjectID != "" {
		var member models.ProjectCollaborator
		if err := db.DB.Where("project_id = ? AND user_id = ?", env.ProjectID, userID).First(&member).Error; err == nil {
			hasAccess = true
		}
	}

	if !hasAccess {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
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
	msgType, _ := data["type"].(string)
	userID, _ := data["user_id"].(string)
	userName, _ := data["user_name"].(string)

	timestamp := time.Now()
	if t, ok := data["timestamp"].(time.Time); ok {
		timestamp = t
	}

	WSHub.broadcast <- BroadcastMessage{
		Type:      msgType,
		EnvID:     envID,
		UserID:    userID,
		UserName:  userName,
		Data:      data,
		Timestamp: timestamp,
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


package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	id     string
	room   string
	isCall bool
}

var (
	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	rooms    = make(map[string]map[*Client]bool) 
)

// Добавить/удалить клиента
func addClient(c *Client) {
	if rooms[c.room] == nil {
		rooms[c.room] = make(map[*Client]bool)
	}
	rooms[c.room][c] = true
}

func removeClient(c *Client) {
	if clients, ok := rooms[c.room]; ok {
		delete(clients, c)
		if len(clients) == 0 {
			delete(rooms, c.room)
		}
	}
	close(c.send)
}


func callCount(room string, exclude *Client) int {
	clients := rooms[room]
	if clients == nil {
		return 0
	}
	count := 0
	for c := range clients {
		if c.isCall && c != exclude {
			count++
		}
	}
	return count
}

func broadcast(room string, data []byte, sender *Client) {
	for c := range rooms[room] {
		if c != sender {
			select {
			case c.send <- data:
			default:
			}
		}
	}
}

func wsHandler(c *gin.Context) {
	room := c.Param("idRoom")
	typeWS := c.Param("typeWS")
	isCall := typeWS == "call"

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{
		conn:   conn,
		send:   make(chan []byte, 256),
		id:     conn.RemoteAddr().String(),
		room:   room,
		isCall: isCall,
	}


	if isCall {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			conn.Close()
			return
		}
		var req struct{ Type string }
		if json.Unmarshal(msg, &req) == nil && req.Type == "checkCountUserCall" {
			count := callCount(room, nil)
			resp, _ := json.Marshal(map[string]interface{}{
				"type":  "checkCountUserCall",
				"count": count,
			})
			conn.WriteMessage(websocket.TextMessage, resp)
		}
	}


	addClient(client)
	fmt.Printf("%s в комнате %s (call=%v). Всего call: %d\n",
		client.id, room, isCall, callCount(room, nil))


	go readPump(client)
	go writePump(client)
}

func readPump(c *Client) {
	defer func() {
		c.conn.Close()
		removeClient(c)
	}()
	for {
		_, msg, err := c.conn.ReadMessage()
		fmt.Println(string(msg))
		if err != nil {
			break
		}
		broadcast(c.room, msg, c)
	}
}

func writePump(c *Client) {
	ticker := time.NewTicker(50 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			c.conn.WriteMessage(websocket.TextMessage, msg)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func main() {
	r := gin.Default()
	r.GET("/ws/:idRoom/:typeWS", wsHandler)
	fmt.Println("🚀 http://localhost:8090/ws/:idRoom/:typeWS")
	r.Run(":8090")
}

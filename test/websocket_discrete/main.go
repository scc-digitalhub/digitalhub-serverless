package main

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	WebSocketAddr = "ws://localhost:9001/ws"
)

func main() {
	wsConn, _, err := websocket.DefaultDialer.Dial(WebSocketAddr, nil)
	if err != nil {
		log.Fatalf("Could not connect to processing WebSocket: %v", err)
	}
	defer wsConn.Close()

	// Read responses from processor
	go func() {
		for {
			_, msg, err := wsConn.ReadMessage()
			if err != nil {
				log.Printf("WebSocket read error: %v", err)
				return
			}
			log.Printf("RESPONSE: %s", string(msg))
		}
	}()

	log.Println("Connected")

	msgs := []string{
		"state = A",
		"state = B",
		"state = C",
		"state = D",
	}

	for _, m := range msgs {
		log.Printf("SEND: %s", m)

		err := wsConn.WriteMessage(websocket.TextMessage, []byte(m))
		if err != nil {
			log.Printf("WebSocket send error: %v", err)
			return
		}

		time.Sleep(2 * time.Second)
	}

	log.Println("Done sending. Waiting for responses...")
	time.Sleep(15 * time.Second)
}

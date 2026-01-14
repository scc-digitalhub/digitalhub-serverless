package main

import (
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/gorilla/websocket"
)

const (
	// change to a relevant rtspURL where the streaming is running
	rtspURL       = "rtsp://localhost:8554/mystream"
	WebSocketAddr = "ws://localhost:9001/ws"
	bufSize       = 4096
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
			log.Printf("Reading from websocket: %s", string(msg))
		}
	}()

	cmd := exec.Command("ffmpeg",
		"-loglevel", "warning",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-i", rtspURL,
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ac", "1",
		"-ar", "16000",
		"pipe:1",
	)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Could not get FFmpeg stdout: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start FFmpeg: %v", err)
	}

	log.Println("FFmpeg started → forwarding PCM to processing WebSocket")

	buf := make([]byte, bufSize)

	for {
		n, err := stdout.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Println("FFmpeg closed stream")
			} else {
				log.Println("FFmpeg read error:", err)
			}
			break
		}

		if n > 0 {
			// send PCM frame
			err := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
			if err != nil {
				log.Printf("WebSocket send error: %v", err)
				break
			}
		}
	}

	cmd.Wait()
	log.Println("Forwarder exiting")
}

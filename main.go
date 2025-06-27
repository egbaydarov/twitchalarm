package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/gorilla/websocket"
)

const (
	wsEndpoint = "wss://irc-ws.chat.twitch.tv:443"
	channel    = "#yegorbaydarov"

	textToSpeechApi = "https://api.elevenlabs.io/v1/text-to-speech/jVJFtjYKoJ2iiMkNqq8P?output_format=mp3_44100_128"
)

func main() {
	nick := fmt.Sprintf("justinfan%d", rand.Intn(99999))
	client := &http.Client{}

	conn, _, err := websocket.DefaultDialer.Dial(wsEndpoint, nil)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// D-Bus connection (maco must be running)
	dbusConn, err := dbus.SessionBus()
	if err != nil {
		log.Fatalf("dbus: %v", err)
	}
	defer dbusConn.Close()

	ircWrite := func(msg string) {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg+"\r\n")); err != nil {
			log.Fatalf("write: %v", err)
		}
	}

	// Anonymous IRC handshake
	ircWrite("PASS SCHMOOPIIE")
	ircWrite("NICK " + nick)
	ircWrite("CAP REQ :twitch.tv/tags")
	ircWrite("JOIN " + channel)

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Fatalf("read: %v", err)
		}
		line := string(raw)

		if strings.HasPrefix(line, "PING") {
			ircWrite("PONG :tmi.twitch.tv")
			continue
		}

		if strings.Contains(line, " PRIVMSG ") {
			// Properly parse tag + prefix + message
			rawNoTags := line
			if strings.HasPrefix(rawNoTags, "@") {
				if tagEnd := strings.Index(rawNoTags, " "); tagEnd != -1 && tagEnd+1 < len(rawNoTags) {
					rawNoTags = rawNoTags[tagEnd+1:]
				}
			}

			if strings.HasPrefix(rawNoTags, ":") {
				prefixEnd := strings.Index(rawNoTags, "!")
				if prefixEnd != -1 {
					username := rawNoTags[1:prefixEnd]

					msgParts := strings.SplitN(rawNoTags, " :", 2)
					if len(msgParts) == 2 {
						message := strings.TrimSpace(msgParts[1])
						sendNotify(client, dbusConn, username, message)
					}
				}
			}
		}
	}
}

func sendNotify(
	client *http.Client,
	conn *dbus.Conn,
	username, text string) {
	if len(text) > 180 {
		text = text[:177] + "..."
	}

	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")

	hints := map[string]dbus.Variant{}

	_ = exec.Command("paplay", "/home/byda/sandbox/twitch-notifs/applepay.wav").Run()

	var id uint32
	call := obj.Call(
		"org.freedesktop.Notifications.Notify", 0,
		"twitch-chat",
		uint32(0),
		"",
		username,
		text,
		[]string{},
		hints,
		int32(7000),
	)
	if call.Err != nil {
		log.Printf("notify error: %v", call.Err)
	}

	_ = call.Store(&id)

	var body io.Reader = strings.NewReader(fmt.Sprintf("{\"text\": \"%s\",\"model_id\": \"eleven_multilingual_v2\"}", text))
	req, _ := http.NewRequest("POST", textToSpeechApi, body)

	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	req.Header.Add("xi-api-key", apiKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("eleven labs api error: %v", call.Err)
		return
	}

	var audio []byte
	resp.Body.Read(audio)
	fileName := fmt.Sprintf("/tmp/%s-%d.mp3", username, time.Now().Unix())
	err = os.WriteFile(fileName, audio, 0644)

	if err != nil {
		log.Printf("save file error: %v", call.Err)
		return
	}

	_ = exec.Command("paplay", fileName).Run()
}

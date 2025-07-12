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

	readMessage(client, "iam", "запустился я")
	showNotificaton(dbusConn, "iam", "запустился я")

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
			// Parse tags
			tags := make(map[string]string)
			rawNoTags := line
			if strings.HasPrefix(line, "@") {
				if tagEnd := strings.Index(line, " "); tagEnd != -1 && tagEnd+1 < len(line) {
					tagStr := line[1:tagEnd]
					rawNoTags = line[tagEnd+1:]
					for _, tag := range strings.Split(tagStr, ";") {
						parts := strings.SplitN(tag, "=", 2)
						if len(parts) == 2 {
							tags[parts[0]] = parts[1]
						}
					}
				}
			}

			if strings.HasPrefix(rawNoTags, ":") {
				prefixEnd := strings.Index(rawNoTags, "!")
				if prefixEnd != -1 {
					username := rawNoTags[1:prefixEnd]

					msgParts := strings.SplitN(rawNoTags, " :", 2)
					if len(msgParts) == 2 {
						message := strings.TrimSpace(msgParts[1])
						// Check if user is a subscriber
						isSubscriber := false
						if badges, ok := tags["badges"]; ok {
							isSubscriber = strings.Contains(badges, "subscriber/")
						}
						sendNotify(client, dbusConn, username, message, isSubscriber)
					}
				}
			}
		}
	}
}

func sendNotify(
	client *http.Client,
	conn *dbus.Conn,
	username, text string,
	isSubscriber bool,
) {
	if len(text) > 400 {
		text = string([]rune(text)[:177]) + "..."
	}

	//Only process text-to-speech for subscribers
	//if !isSubscriber {
	//  _ = exec.Command("paplay", "/home/byda/sandbox/twitch-notifs/applepay.wav").Run()
	//	return
	//} else {}

	readMessage(client, username, text)
	showNotificaton(conn, username, text)
}

func showNotificaton(
	conn *dbus.Conn,
	username, text string,
) {
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	hints := map[string]dbus.Variant{}
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
}

func readMessage(
	client *http.Client,
	username, text string,
) {
	var body io.Reader = strings.NewReader(fmt.Sprintf("{\"text\": \"%s\",\"model_id\": \"eleven_multilingual_v2\"}", text))
	req, _ := http.NewRequest("POST", textToSpeechApi, body)

	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	req.Header.Add("xi-api-key", apiKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("eleven labs api error: %v", err)
		return
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("read body error: %v", err)
		return
	}

	err = os.MkdirAll("/tmp/twitchmessages_audio", 0755)
	if err != nil {
		log.Printf("error creating directory: %v", err)
		return
	}

	fileName := fmt.Sprintf("/tmp/twitchmessages_audio/%s-%d.mp3", username, time.Now().Unix())
	err = os.WriteFile(fileName, audio, 0644)

	if err != nil {
		log.Printf("save file error: %v", err)
		return
	}
	_ = exec.Command("paplay", fileName).Run()
}

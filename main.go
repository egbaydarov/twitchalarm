package main

import (
    "fmt"
    "log"
    "math/rand"
    "os/exec"
    "strings"
    "time"

    "github.com/gorilla/websocket"
)

const (
    wsEndpoint = "wss://irc-ws.chat.twitch.tv:443"
    channel    = "#yegorbaydarov"
)

func main() {
    rand.Seed(time.Now().UnixNano())
    nick := fmt.Sprintf("justinfan%d", rand.Intn(99999))

    dialer, _, err := websocket.DefaultDialer.Dial(wsEndpoint, nil)
    if err != nil {
        log.Fatalf("dial: %v", err)
    }
    defer dialer.Close()

    ircWrite := func(msg string) {
        if err := dialer.WriteMessage(websocket.TextMessage, []byte(msg+"\r\n")); err != nil {
            log.Fatalf("write: %v", err)
        }
    }

    // Minimal IRC handshake (anonymous)
    ircWrite("PASS SCHMOOPIIE")            // arbitrary pass is fine
    ircWrite("NICK " + nick)               // anonymous nick
    ircWrite("CAP REQ :twitch.tv/tags")    // want tags so we can ignore them
    ircWrite("JOIN " + channel)

    for {
        _, raw, err := dialer.ReadMessage()
        if err != nil {
            log.Fatalf("read: %v", err)
        }
        line := string(raw)

        if strings.HasPrefix(line, "PING") {
            ircWrite("PONG :tmi.twitch.tv")
            continue
        }

        if i := strings.Index(line, " PRIVMSG "); i != -1 {
            msgParts := strings.SplitN(line, " :", 3)
            if len(msgParts) >= 3 {
                message := strings.TrimSpace(msgParts[2])
                spawnNotify(message)
            }
        }
    }
}

func spawnNotify(text string) {
    if len(text) > 180 {
        text = text[:177] + "..."
    }
    cmd := exec.Command("hyprctl", "notify", "-1", "10000", "rgb(ff0000)", fmt.Sprintf("fontsize:35 %s", text))
    _ = cmd.Run()
}

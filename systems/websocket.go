package systems

import (
	"log"
	"log/slog"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func StartListener(outbox chan Killmail, stop chan struct{}, errchan chan error) error {
	// connect to websocket
	u := url.URL{Scheme: "wss", Host: "zkillboard.com", Path: "/websocket/"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	if err := c.WriteJSON(map[string]string{
		"action":  "sub",
		"channel": "killstream",
	}); err != nil {
		slog.Warn("failed to subscribe to killstream", "error", err)
		return err
	}

	// start heartbeat ticker
	heartbeat := time.NewTicker(30 * time.Second)
	var killmail Killmail

	// listener loop in goroutine
	// 	send killmails to outbox
	errorCount := 0
	go func() {
		defer close(done)
		for {
			err := c.ReadJSON(&killmail)
			if err != nil {
				if _, ok := err.(*websocket.CloseError); ok {
					slog.Info("websocket connection closed")
					return
				}
				errchan <- err
				slog.Warn("error reading message", "error", err)
				errorCount++
				if errorCount > 5 {
					slog.Error("too many errors, exiting")
					return
				}
				continue
			}

			errorCount = 0

			if killmail.Zkill.NPC {
				slog.Debug("filtered out killmail",
					"reason", "NPC kill",
					"id", killmail.KillmailID,
				)
				continue
			}

			if !filter(killmail) {
				slog.Debug("filtered out killmail",
					"reason", "system is on ignore list",
					"id", killmail.KillmailID,
					"system", killmail.SolarSystemID,
				)
				continue
			}

			slog.Info("received new killmail",
				"original_timestamp", killmail.OriginalTimestamp,
				"id", killmail.KillmailID,
				"hash", killmail.Zkill.Hash,
				"url", killmail.Zkill.URL,
			)

			outbox <- killmail
		}
	}()

	for {
		select {
		case <-stop:
			slog.Info("stopping websocket listener")
			heartbeat.Stop()
			if err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
				slog.Warn("failed to send close message", "error", err)
			}
			<-done
			return nil
		case <-heartbeat.C:
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return err
			}
		}
	}
}

func filter(km Killmail) bool {
	valid := true

	Register().mx.Lock()
	systems := Register().systems
	Register().mx.Unlock()

	found := false
	for _, sys := range systems {
		if sys.SolarSystemID == km.SolarSystemID {
			found = true
		}
	}
	if !found {
		return false
	}

	return valid
}

package systems

import (
	"log/slog"

	"github.com/gorilla/websocket"
)

type Killmail struct {
	Zkill struct {
		URL string `json:"url"`
	} `json:"zkb"`
}

type Listener struct {
	// Systems is a snapshot of the systems this instance is listening for
	Systems []System

	// Stop is a channel to stop the listener
	Stop chan struct{}

	connection *websocket.Conn
}

func NewListener(systems []System) (*Listener, error) {
	ws, _, err := websocket.DefaultDialer.Dial("wss://zkillboard.com/websocket/", nil)
	if err != nil {
		return nil, err
	}

	return &Listener{
		Systems:    systems,
		Stop:       make(chan struct{}),
		connection: ws,
	}, nil
}

func (l *Listener) Start(outbox chan Killmail, done chan struct{}, errors chan error) {
	defer close(done)
	filters := buildFilters(l.Systems)

	slog.Debug("subscribing to channel", "channel", filters)

	if err := l.connection.WriteJSON(filters); err != nil {
		errors <- err
	}

	go func() {
		for {
			var msg Killmail
			if err := l.connection.ReadJSON(&msg); err != nil {
				slog.Error("failed to receive message", "error", err)
				if err == websocket.ErrCloseSent {
					break
				}
			}

			outbox <- msg
		}
	}()

	<-l.Stop

	err := l.connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		errors <- err
	}

	slog.Debug("websocket listener stopped")
}

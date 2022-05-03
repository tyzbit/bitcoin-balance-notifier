package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	TimeFormatter        string = time.RFC3339
)

// Info is an interface for AddressInfo and PubkeyInfo
type Info interface {
	GetIdentifier() string
	GetNickname() string
	Update(Watcher) error
}

// UpdateInfo calls Update() for the provided Info interface
func (w Watcher) UpdateInfo(i Info) {
	if err := i.Update(w); err != nil {
		log.Errorf("error updating address info for \"%s\" (%s): %v",
			i.GetNickname(), i.GetIdentifier(), err)
	}
}

// AddCancelSignal adds a cancel channel to w.CancelSignals
func (w Watcher) AddCancelSignal(i string, c chan bool) {
	w.CancelSignals[i] = c
	w.CancelWaitGroup.Done()
}

// DeleteCancelSignal deletes the cancel channel on w.CancelSignals
func (w Watcher) DeleteCancelSignal(i string) {
	// Only send to the channel if it's open and buffered
	if w.CancelSignals[i] != make(chan bool, 1) {
		w.CancelSignals[i] <- true
	}
	delete(w.CancelSignals, i)
	w.CancelWaitGroup.Done()
}

// SendNotification sends a message filled with values from a template
// to Discord
func (w Watcher) SendNotification(i interface{}, mt string) {
	t, err := template.New("addressMessage").Parse(mt)
	if err != nil {
		log.Error("error setting up template: ", err)
		return
	}

	var b bytes.Buffer
	err = t.Execute(&b, i)
	if err != nil {
		log.Errorf("error executing template, err: %v", err)
		return
	}

	message := b.String()
	payload := DiscordPayload{
		Content: message,
	}
	var m bytes.Buffer
	err = json.NewEncoder(&m).Encode(payload)
	if err != nil {
		log.Errorf("unable to encode payload: %+v", payload)
		return
	}

	client := http.Client{}
	resp, respErr := client.Post(w.Config.DiscordWebhook, "application/json", &m)
	if respErr != nil || resp.StatusCode != 204 {
		log.Errorf("error calling Discord API (%s): %v", resp.Status, respErr)
		return
	}
}

// pkg/notify/notifier.go

package notify

import (
	"fmt"
	"net/http"
	"net/url"
)

type Notifier struct {
	appToken    string
	userKey     map[string]string
	pushoverURL string
}

func NewNotifier(appToken string, userKey map[string]string) *Notifier {
	return &Notifier{
		appToken:    appToken,
		userKey:     userKey,
		pushoverURL: "https://api.pushover.net/1/messages.json",
	}
}

func (n *Notifier) Send(message string) error {
	resp, err := http.PostForm(n.pushoverURL, url.Values{
		"token":   {n.appToken},
		"user":    {n.userKey["aaryan"]},
		"message": {message},
	})
	if err != nil {
		return fmt.Errorf("pushover request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pushover returned status %d", resp.StatusCode)
	}

	return nil
}

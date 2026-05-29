package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Telegram struct {
	BotToken string
	ChatID   string
	HTTP     *http.Client
}

func NewTelegram(token, chatID string) *Telegram {
	return &Telegram{
		BotToken: token,
		ChatID:   chatID,
		HTTP:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *Telegram) Send(ctx context.Context, text string) error {
	if t.BotToken == "" || t.ChatID == "" {
		return errors.New("telegram: missing bot token or chat id")
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	form := url.Values{}
	form.Set("chat_id", t.ChatID)
	form.Set("text", text)
	form.Set("disable_web_page_preview", "true")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("telegram: http %d", resp.StatusCode)
	}
	return nil
}

// BotInfo returns the bot's username so we can show a t.me/<username> link.
func BotInfo(ctx context.Context, token string) (username string, err error) {
	if token == "" {
		return "", errors.New("telegram: empty bot token")
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("telegram getMe: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if !parsed.OK || parsed.Result.Username == "" {
		return "", errors.New("telegram getMe: bot info missing")
	}
	return parsed.Result.Username, nil
}

// DiscoverChatID polls getUpdates and returns the chat_id of the most recent
// message sent to the bot. The user is expected to open the bot in Telegram
// and send any message (e.g. /start) before this is called.
//
// Returns ("", nil) when the bot has no updates yet — the caller should
// prompt the user to send a message and retry.
func DiscoverChatID(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", errors.New("telegram: empty bot token")
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("telegram getUpdates: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		OK     bool `json:"ok"`
		Result []struct {
			Message struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
			ChannelPost struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"channel_post"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if !parsed.OK {
		return "", errors.New("telegram getUpdates: ok=false")
	}
	// Walk from newest to oldest. Prefer private/group messages over channel posts.
	for i := len(parsed.Result) - 1; i >= 0; i-- {
		if id := parsed.Result[i].Message.Chat.ID; id != 0 {
			return strconv.FormatInt(id, 10), nil
		}
		if id := parsed.Result[i].ChannelPost.Chat.ID; id != 0 {
			return strconv.FormatInt(id, 10), nil
		}
	}
	return "", nil
}

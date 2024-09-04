package backend

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/itsByte/gomarkov"
	tele "gopkg.in/telebot.v3"
)

var (
	ChainOrder = flag.Int("order", 1, "Sets Markov chain order")
)

const (
	baseDataPath string        = "data"
	oldThreshold time.Duration = 24 * time.Hour
)

type TimedChain struct {
	access time.Time
	chain  *gomarkov.Chain
}

type ChainOutput struct {
	Ty   string
	Id   string
	Text string
}

type Tables map[tele.ChatID]TimedChain

func (t Tables) getOrCreate(cID tele.ChatID) (gomarkov.Chain, error) {
	tc, exists := t[cID]
	if exists {
		t[cID] = TimedChain{time.Now(), tc.chain}
		return *tc.chain, nil
	}
	filePath := baseDataPath + "/" + strconv.Itoa(int(cID)) + ".json"
	if _, err := os.Stat(filePath); err != nil {
		slog.Info("Creating new table for", "chatID", cID)

		c := gomarkov.NewChain(*ChainOrder)
		t[cID] = TimedChain{time.Now(), c}
		return *c, nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return gomarkov.Chain{}, err
	}
	c := gomarkov.NewChain(*ChainOrder)
	err = c.UnmarshalJSON(data)
	if err != nil {
		return gomarkov.Chain{}, err
	}
	t[cID] = TimedChain{time.Now(), c}
	return *c, nil
}

func ProcessMessage(t Tables, context tele.Context, ty string) error {
	cID := context.Chat().ID
	c, err := t.getOrCreate(tele.ChatID(cID))
	if err != nil {
		return err
	}
	msg := []string{ty}
	if ty != "\u001F_TEXT" {
		msg = append(msg, context.Message().Media().MediaFile().FileID)
	}
	msg = append(msg, strings.Split(context.Text(), " ")...)
	slog.Debug("Training for chat", "chatID", cID)
	c.Add(msg)
	return nil
}

func GenerateMessage(t Tables, context tele.Context) (ChainOutput, error) {
	cID := context.Chat().ID
	c, err := t.getOrCreate(tele.ChatID(cID))
	slog.Debug("Generating for", "chatID", cID, "order", c.Order)
	if err != nil {
		return ChainOutput{}, err
	}
	msg, err := c.GenerateAllLimited(500)
	if err != nil {
		return ChainOutput{}, err
	}
	switch msg[0] {
	case "\u001F_TEXT":
		{
			return ChainOutput{Ty: msg[0], Text: strings.Join(msg[1:], " ")}, err
		}
	case "\u001F_PHOTO", "\u001F_ANIMATION":
		{
			return ChainOutput{Ty: msg[0], Id: msg[1], Text: strings.Join(msg[2:], " ")}, err
		}
	case "\u001F_STICKER":
		{
			return ChainOutput{Ty: msg[0], Id: msg[1]}, err
		}
	default:
		{
			return ChainOutput{Ty: "\u001F_TEXT", Text: strings.Join(msg, " ")}, err
		}
	}
}

func (t Tables) Persist() error {
	os.MkdirAll(baseDataPath, 0755)
	for cID, tc := range t {
		m := tc.chain
		data, err := m.MarshalJSON()
		if err != nil {
			return err
		}
		filePath := baseDataPath + "/" + strconv.Itoa(int(cID)) + ".json"
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		n, err := file.Write(data)
		if err != nil {
			return err
		}
		err = file.Truncate(int64(n))
		if err != nil {
			return err
		}
	}
	return nil
}

func (t Tables) UnloadOld() {
	t.Persist()
	oldTime := time.Now().Add(-oldThreshold)
	for k, v := range t {
		if v.access.Before(oldTime) {
			slog.Debug("Table unloaded", "chatID", k, "lastAccess", v.access)
			delete(t, k)
		}
	}
}

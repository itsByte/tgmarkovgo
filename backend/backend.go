package backend

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mb-14/gomarkov"
	tele "gopkg.in/telebot.v3"
)

var (
	chainOrder = flag.Int("order", 1, "Sets Markov chain order")
)

const (
	baseDataPath string        = "data"
	oldThreshold time.Duration = 24 * time.Hour
)

type TimedChain struct {
	access time.Time
	chain  *gomarkov.Chain
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

		c := gomarkov.NewChain(*chainOrder)
		t[cID] = TimedChain{time.Now(), c}
		return *c, nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return gomarkov.Chain{}, err
	}
	c := gomarkov.NewChain(*chainOrder)
	err = c.UnmarshalJSON(data)
	if err != nil {
		return gomarkov.Chain{}, err
	}
	t[cID] = TimedChain{time.Now(), c}
	return *c, nil
}

func ProcessMessage(t Tables, context tele.Context) error {
	cID := context.Chat().ID
	c, err := t.getOrCreate(tele.ChatID(cID))
	if err != nil {
		return err
	}
	slog.Debug("Training for chat", "chatID", cID)
	c.Add(strings.Split(context.Text(), " "))
	return nil
}

func GenerateMessage(t Tables, context tele.Context) (string, error) {
	cID := context.Chat().ID
	slog.Debug("Generating for", "chatID", cID)
	c, err := t.getOrCreate(tele.ChatID(cID))
	if err != nil {
		return "", err
	}
	generatedText := []string{}
	current := make(gomarkov.NGram, 0)
	for i := 0; i < c.Order; i++ {
		current = append(current, gomarkov.StartToken)
	}
	for {
		next, err := c.Generate(current)
		if err != nil {
			return "", err
		}
		if next == gomarkov.EndToken {
			break
		}

		current = append(current, next)[1:]
		generatedText = append(generatedText, next)
	}
	return strings.Join(generatedText, " "), nil
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

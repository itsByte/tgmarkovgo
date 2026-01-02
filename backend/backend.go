package backend

import (
	"flag"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/itsByte/gomarkov"
	tele "gopkg.in/telebot.v3"
)

var (
	ChainOrder = flag.Int("order", 1, "Sets Markov chain order")
)

var (
	BaseDataPath = flag.String("datadir", "data", "Data Directory")
)

type ChainOutput struct {
	Ty   string // Type of message
	Id   string // ID of media message
	Text string // Text content of message
}

func BuildChain() (*gomarkov.Chain, error) {
	// Create a new storage backend and chain
	storage, err := gomarkov.NewPebbleStorage(path.Join(*BaseDataPath, "db"))
	if err != nil {
		panic(fmt.Errorf("failed to create pebble storage: %w", err))
	}
	return gomarkov.NewChain(*ChainOrder, storage), nil
}

func ProcessMessage(chain *gomarkov.Chain, context tele.Context, ty string) error {
	cID := context.Chat().ID
	msg := []string{ty}
	if ty != "\u001F_TEXT" {
		msg = append(msg, context.Message().Media().MediaFile().FileID)
	}
	msg = append(msg, strings.Split(context.Text(), " ")...)
	slog.Debug("Training for chat", "chatID", cID)
	chain.Add(cID, msg)
	return nil
}

func GenerateMessage(chain *gomarkov.Chain, context tele.Context) (ChainOutput, error) {
	cID := context.Chat().ID
	slog.Debug("Generating for", "chatID", cID, "order", chain.Order)
	msg, err := chain.GenerateAllLimited(cID, 500)
	if err != nil || len(msg) == 0 {
		return ChainOutput{Ty: "\u001F_TEXT", Text: "Chain is empty!"}, err
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

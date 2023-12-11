package bot

import (
	"flag"
	"itsbyte/markovbotgo/backend"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

var (
	chattiness  = flag.Float64("chattiness", 0.1, "Sets chattiness variable, 0-1")
	replyChance = flag.Float64("replyChance", 0.6, "Sets replyChance variable, 0-1")
)

func Init(t backend.Tables) {
	pref := tele.Settings{
		Token:       os.Getenv("TOKEN"),
		Poller:      &tele.LongPoller{Timeout: 10 * time.Second},
		Synchronous: true,
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		slog.Error("Error", "Code", err)
		return
	}

	b.Handle("/generate", func(c tele.Context) error {
		msg, err := backend.GenerateMessage(t, c)
		if err != nil {
			slog.Error("Error", "Code", err)
			return err
		}
		return c.Send(msg)
	})

	b.Handle(tele.OnText, func(context tele.Context) error {

		err := backend.ProcessMessage(t, context)
		if err != nil {
			slog.Error("Error", "Code", err)
			return err
		}
		willReply := rand.Float64() < *replyChance
		isReply := context.Message().IsReply()
		var isMe bool
		if isReply {
			isMe = context.Message().ReplyTo.Sender.ID == context.Bot().Me.ID
		}
		textMentionsMe := strings.Contains(context.Text(), b.Me.Username)
		if willReply &&
			((isReply && isMe) ||
				textMentionsMe) {
			msg, err := backend.GenerateMessage(t, context)
			if err != nil {
				slog.Error("Error", "Code", err)
				return err
			}
			return context.Reply(msg)
		} else if rand.Float64() < *chattiness {
			msg, err := backend.GenerateMessage(t, context)
			if err != nil {
				slog.Error("Error", "Code", err)
				return err
			}
			return context.Send(msg)
		}

		return nil
	})

	b.Start()
}

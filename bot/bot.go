package bot

import (
	"itsbyte/markovbotgo/backend"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

const (
	chattiness  (float64) = 0.1
	replyChance (float64) = 0.6
)

func Init(t backend.Tables) {
	pref := tele.Settings{
		Token:  os.Getenv("TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	b.Handle("/generate", func(c tele.Context) error {
		msg, err := backend.GenerateMessage(t, c)
		if err != nil {
			log.Fatal(err)
			return err
		}
		return c.Send(msg)
	})

	b.Handle(tele.OnText, func(context tele.Context) error {

		err := backend.ProcessMessage(t, context)
		if err != nil {
			log.Fatal(err)
			return err
		}
		if rand.Float64() < replyChance && ((context.Message().IsReply() && context.Message().ReplyTo.Sender.ID == context.Bot().Me.ID) || (strings.Contains(context.Text(), b.Me.Username))) {
			msg, err := backend.GenerateMessage(t, context)
			if err != nil {
				log.Fatal(err)
				return err
			}
			return context.Reply(msg)
		} else if rand.Float64() < chattiness {
			msg, err := backend.GenerateMessage(t, context)
			if err != nil {
				log.Fatal(err)
				return err
			}
			return context.Send(msg)
		}

		return nil
	})

	b.Start()
}

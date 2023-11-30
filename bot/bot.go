package bot

import (
	"flag"
	"itsbyte/markovbotgo/backend"
	"log"
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
		if rand.Float64() < *replyChance && ((context.Message().IsReply() && context.Message().ReplyTo.Sender.ID == context.Bot().Me.ID) || (strings.Contains(context.Text(), b.Me.Username))) {
			msg, err := backend.GenerateMessage(t, context)
			if err != nil {
				log.Fatal(err)
				return err
			}
			return context.Reply(msg)
		} else if rand.Float64() < *chattiness {
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

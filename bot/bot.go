package bot

import (
	"flag"
	"log/slog"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/itsByte/tgmarkovgo/backend"

	tele "gopkg.in/telebot.v3"
)

var (
	Chattiness  = flag.Float64("chattiness", 0.1, "Sets chattiness variable, 0-1")
	ReplyChance = flag.Float64("replyChance", 0.6, "Sets replyChance variable, 0-1")
)

func doesReply(context tele.Context) bool {
	willReply := rand.Float64() < *ReplyChance
	isReply := context.Message().IsReply()
	var isMe bool
	if isReply {
		isMe = context.Message().ReplyTo.Sender.ID == context.Bot().Me.ID
	}
	textMentionsMe := strings.Contains(context.Text(), context.Bot().Me.Username)
	return willReply && ((isReply && isMe) || textMentionsMe)
}

func processGen(co backend.ChainOutput) any {
	switch co.Ty {
	case "\u001F_TEXT":
		{
			return co.Text
		}
	case "\u001F_PHOTO":
		{
			return &tele.Photo{File: tele.File{FileID: co.Id}, Caption: co.Text}
		}
	case "\u001F_ANIMATION":
		{
			return &tele.Animation{File: tele.File{FileID: co.Id}, Caption: co.Text}
		}
	case "\u001F_STICKER":
		{
			return &tele.Sticker{File: tele.File{FileID: co.Id}}
		}
	default:
		{
			return nil
		}
	}
}

func handleMessage(t backend.Tables, context tele.Context, mutedChats []int64) error {
	if slices.Contains(mutedChats, context.Chat().ID) {
		return nil
	}
	if doesReply(context) {
		co, err := backend.GenerateMessage(t, context)
		if err != nil {
			slog.Error("Error", "Code", err)
			return err
		}
		return context.Reply(processGen(co))
	} else if rand.Float64() < *Chattiness {
		co, err := backend.GenerateMessage(t, context)
		if err != nil {
			slog.Error("Error", "Code", err)
			return err
		}
		return context.Send(processGen(co))
	}
	return nil
}

func removeMute(mutedChats []int64, cID int64) {
	i := slices.Index(mutedChats, cID)
	if i != -1 {
		mutedChats = slices.Delete(mutedChats, i, i+1)
	}
	slog.Info("Unmuting chat", "chatID", cID)
}

func Init(t backend.Tables) {
	pref := tele.Settings{
		Token:       os.Getenv("TOKEN"),
		Poller:      &tele.LongPoller{Timeout: 10 * time.Second},
		Synchronous: true,
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		slog.Error("Error", "Code", err)
		os.Exit(1)
	}

	mutedChats := make([]int64, 0)

	b.Handle("/generate", func(c tele.Context) error {
		co, err := backend.GenerateMessage(t, c)
		if err != nil {
			slog.Error("Error", "Code", err)
			return err
		}
		return c.Send(processGen(co))
	})

	b.Handle("/start", func(c tele.Context) error {
		return c.Send("Hi!")
	})

	b.Handle("/help", func(c tele.Context) error {
		return c.Send("Available commands:\n/start: Starts the bot\n/generate: Generate a new message\n/shut: Stop generating messages in this chat\n/unshut: Resume generating messages in this chat\n/help: Get help about the bot and the commands")
	})

	b.Handle("/shut", func(c tele.Context) error {
		if !slices.Contains(mutedChats, c.Chat().ID) {
			mutedChats = append(mutedChats, c.Chat().ID)
		}
		rTimer := time.NewTimer(30 * time.Minute)
		go func() {
			<-rTimer.C
			removeMute(mutedChats, c.Chat().ID)
		}()
		slog.Info("Muting chat", "chatID", c.Chat().ID)
		return c.Reply("meow...")
	})

	b.Handle("/unshut", func(c tele.Context) error {
		removeMute(mutedChats, c.Chat().ID)
		return c.Reply("yay")
	})

	b.Handle(tele.OnText, func(context tele.Context) error {
		err := backend.ProcessMessage(t, context, "\u001F_TEXT")
		if err != nil {
			slog.Error("Error", "Code", err)
			return err
		}
		return handleMessage(t, context, mutedChats)
	})

	b.Handle(tele.OnPhoto, func(context tele.Context) error {
		err := backend.ProcessMessage(t, context, "\u001F_PHOTO")
		if err != nil {
			slog.Error("Error", "Code", err)
			return err
		}
		return handleMessage(t, context, mutedChats)
	})

	b.Handle(tele.OnAnimation, func(context tele.Context) error {
		err := backend.ProcessMessage(t, context, "\u001F_ANIMATION")
		if err != nil {
			slog.Error("Error", "Code", err)
			return err
		}
		return handleMessage(t, context, mutedChats)
	})

	b.Handle(tele.OnSticker, func(context tele.Context) error {
		err := backend.ProcessMessage(t, context, "\u001F_STICKER")
		if err != nil {
			slog.Error("Error", "Code", err)
			return err
		}
		return handleMessage(t, context, mutedChats)
	})

	b.Start()
}

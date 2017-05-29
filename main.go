package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/telegram-bot-api.v4"
)

type (
	MsgResponse struct {
		chatId    int64
		messageId int
		text      string
	}
	MsgRequest struct {
		chatId    int64
		messageId int
		text      string
	}
)

var (
	configFile string
	chnResp    = make(chan MsgResponse, 5)
	chnReq     = make(chan MsgRequest, 5)
)

func main() {
	flag.StringVar(&configFile, "configFile", "config.yml", "configuration file")
	flag.Parse()

	viper.SetConfigType("yaml")
	viper.SetConfigFile(configFile)
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %s \n\n", err)
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	go getResp()

	botApiKey := viper.GetString("api_key")

	bot, err := tgbotapi.NewBotAPI(botApiKey)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:
			chatID := update.Message.Chat.ID
			msgID := update.Message.MessageID
			msgText := update.Message.Text

			chnReq <- MsgRequest{chatID, msgID, msgText}

		case resp := <-chnResp:
			msg := tgbotapi.NewMessage(resp.chatId, resp.text)
			msg.ReplyToMessageID = resp.messageId
			bot.Send(msg)
		}
	}
}

func getResp() {
	// goroutine for magic
	for msg := range chnReq {
		chnResp <- MsgResponse{msg.chatId, msg.messageId, msg.text}
	}
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/telegram-bot-api.v4"
)

type (
	msgResponse struct {
		chatID    int64
		messageID int
		text      string
	}
	msgRequest struct {
		chatID    int64
		messageID int
		text      string
	}
	issRespJSON struct {
		Securities struct {
			Columns []string   `json:"columns"`
			Data    [][]string `json:"data"`
		} `json:"securities"`
	}
)

var (
	myClient           = &http.Client{Timeout: 2 * time.Second}
	issURLSearchJSON   = "https://iss.moex.com/iss/securities.json?q="
	issURLSearchParams = "&iss.only=marketdata&iss.meta=off&securities.columns=secid,name"
	issURLImg          = "https://iss.moex.com/cs/engines/stock/markets/index/securities/"

	configFile string
	chnResp    = make(chan msgResponse, 5)
	chnReq     = make(chan msgRequest, 5)
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

	botAPIKey := viper.GetString("api_key")

	bot, err := tgbotapi.NewBotAPI(botAPIKey)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, _ := bot.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:

			if update.Message != nil && !(update.Message.Chat.IsGroup() || update.Message.Chat.IsSuperGroup()) {
				chatID := update.Message.Chat.ID
				msgID := update.Message.MessageID
				msgText := update.Message.Text

				chnReq <- msgRequest{chatID, msgID, msgText}
			}

			if update.Message == nil && update.InlineQuery != nil {
				var articles []interface{}
				issResponse := issRespJSON{}

				query := update.InlineQuery.Query
				getJSON(issURLSearchJSON+query+issURLSearchParams, &issResponse)

				fmt.Println(issResponse)

				if len(issResponse.Securities.Data) == 0 {
					msg := tgbotapi.NewInlineQueryResultArticle(update.InlineQuery.ID, "No one index matches", "No one index matches")
					articles = append(articles, msg)
				} else {
					for i, indx := range issResponse.Securities.Data {
						text := fmt.Sprintf("*%s*"+
							"[.](%s)\n",
							indx[0]+" - "+indx[1],
							issURLImg+indx[0])

						msg := tgbotapi.NewInlineQueryResultArticleMarkdown(indx[0], indx[0]+" - "+indx[1], text)
						articles = append(articles, msg)
						if i >= 6 {
							break
						}
					}
				}

				inlineConfig := tgbotapi.InlineConfig{
					InlineQueryID: update.InlineQuery.ID,
					IsPersonal:    true,
					CacheTime:     0,
					Results:       articles,
				}

				_, err := bot.AnswerInlineQuery(inlineConfig)
				if err != nil {
					log.Println(err)
				}
			}

		case resp := <-chnResp:
			msg := tgbotapi.NewMessage(resp.chatID, resp.text)
			msg.ReplyToMessageID = resp.messageID
			bot.Send(msg)
		}
	}
}

func getResp() {
	// goroutine for magic
	for msg := range chnReq {
		chnResp <- msgResponse{msg.chatID, msg.messageID, msg.text}
	}
}

func getJSON(url string, target interface{}) error {
	r, err := myClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

package main

import (
	"bn-service/contracts"
	"bn-service/models"
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/osteele/liquid"
	"github.com/sarulabs/di/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
	"path"
)

var PgsqlDef = &di.Def{
	Name:  "pgsql",
	Scope: di.App,
	Build: func(ctn di.Container) (interface{}, error) {
		db, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{})
		if err != nil {
			panic("failed to connect database")
		}
		fmt.Println("Database ok.")
		return db, nil
	},
}

var TgDef = &di.Def{
	Name:  "tg",
	Scope: di.App,
	Build: func(ctn di.Container) (interface{}, error) {
		bot, err := tgbotapi.NewBotAPI(os.Getenv("TG_BOT_TOKEN"))
		if err != nil {
			log.Panic(err)
		}

		return bot, err
	},
}

var TgUpdaterDef = &di.Def{
	Name:  "tg-updater",
	Scope: di.App,
	Build: func(ctn di.Container) (interface{}, error) {
		db := ctn.Get("pgsql").(*gorm.DB)
		bot := ctn.Get("tg").(*tgbotapi.BotAPI)

		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates := bot.GetUpdatesChan(u)

		fmt.Println("Telegram ok.")

		for update := range updates {
			if update.Message == nil {
				continue
			}
			if !update.Message.IsCommand() {
				continue
			}

			switch update.Message.Command() {
			case "start":
				data := models.Subscribers{
					ChatId: update.Message.Chat.ID,
				}
				db.FirstOrCreate(&data)
				bot.Send(tgbotapi.MessageConfig{
					BaseChat: tgbotapi.BaseChat{
						ChatID:           update.Message.Chat.ID,
						ReplyToMessageID: 0,
					},
					Text:                  renderMessage("subscription_success", map[string]string{}),
					DisableWebPagePreview: false,
					ParseMode:             "MarkdownV2",
				})
			}
		}

		return updates, nil
	},
}

func main() {
	godotenv.Load()

	builder, _ := di.NewEnhancedBuilder()
	builder.Add(PgsqlDef)
	builder.Add(TgDef)
	builder.Add(TgUpdaterDef)
	ctn, _ := builder.Build()
	defer ctn.Delete()

	nc, err := nats.Connect(os.Getenv("NATS_HOST"))
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	fmt.Println("NATS ok.")

	_, err = nc.Subscribe(string(contracts.SubjectCustomerRegistered), func(msg *nats.Msg) {
		handleNATSMessages(msg, ctn)
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to subject: %v", err)
	}

	_ = ctn.Get("tg-updater").(*gorm.DB)

	select {}
}

func handleNATSMessages(msg *nats.Msg, ctn di.Container) {
	db := ctn.Get("pgsql").(*gorm.DB)

	switch msg.Subject {
	case string(contracts.SubjectCustomerRegistered):
		var message contracts.CustomerRegisteredEvent
		err := json.Unmarshal(msg.Data, &message)
		if err != nil {
			// TODO: Sentry.
			log.Panic(err)
			break
		}
		data := models.Event{
			Subject: msg.Subject,
			Payload: message.Data.Payload,
		}
		db.Create(&data)
		text := renderMessage("new_registration", map[string]string{
			"firstname": message.Data.Payload.Firstname,
			"lastname":  message.Data.Payload.Lastname,
			"language":  message.Data.Payload.Language,
			"country":   message.Data.Payload.Country,
		})
		sendMessage(text, ctn)
		break
	}
}

func sendMessage(text string, ctn di.Container) {
	db := ctn.Get("pgsql").(*gorm.DB)
	bot := ctn.Get("tg").(*tgbotapi.BotAPI)
	subscribers, _ := db.Model(&models.Subscribers{}).Rows()

	for subscribers.Next() {
		var sub models.Subscribers
		db.ScanRows(subscribers, &sub)

		bot.Send(tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           sub.ChatId,
				ReplyToMessageID: 0,
			},
			Text:                  text,
			DisableWebPagePreview: false,
			ParseMode:             "MarkdownV2",
		})
	}
}

func renderMessage(tpl string, ctx map[string]string) string {
	engine := liquid.NewEngine()

	template, err := os.ReadFile(path.Join("templates", tpl+".md"))
	if err != nil {
		log.Fatalln(err)
	}

	out, err := engine.ParseAndRenderString(string(template), map[string]interface{}{
		"m": ctx,
	})
	if err != nil {
		log.Fatalln(err)
	}

	return out
}

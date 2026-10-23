package main

//go:generate go run github.com/AshokShau/gotdbot/scripts/tools

import (
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/AshokShau/gotdbot"
	"github.com/AshokShau/gotdbot/filters/callbackquery"
	"github.com/AshokShau/gotdbot/filters/message"
)

var started = time.Now()

func main() {
	if err := loadConfig(); err != nil {
		log.Fatal(err)
	}
	for _, tool := range []string{"mongodump", "mongorestore"} {
		if _, err := exec.LookPath(tool); err != nil {
			panic(err.Error())
		}
	}

	if err := os.MkdirAll("backups", 0700); err != nil {
		log.Fatal(err)
	}

	opts := gotdbot.DefaultClientConfig()
	opts.ParseMode = gotdbot.ParseModeHTML
	opts.UpdateWorkers = 2

	c, err := gotdbot.NewClient(apiID, apiHash, token, opts)
	if err != nil {
		panic(err.Error())
	}

	c.OnCommand("start", startCommand).AddAliases("help").AddFilter(message.Incoming)
	c.OnCommand("ping", pingCommand).AddFilter(message.Incoming)
	c.OnCommand("privacy", privacyCommand).AddFilter(message.Incoming)
	c.OnCommand("mongo", mongoCommand).AddFilter(gotdbot.MessageFilter(message.Private).And(message.Incoming))
	c.OnMessage(mongoCommand, gotdbot.MessageFilter(message.Private).And(message.Incoming).And(message.Regex(`^(?:mongo(?:db)?\s+|mongodb(?:\+srv)?://)`)))
	c.OnUpdateNewCallbackQuery(mongoCallback, callbackquery.Prefix("mongo:"))
	c.OnUpdateNewCallbackQuery(helpCallback, callbackquery.Equal("help"))
	c.OnUpdateNewCallbackQuery(privacyCallback, callbackquery.Equal("privacy"))

	if err = c.Start(); err != nil {
		c.Logger.Warnf("Telegram login failed; %v", err)
		c.Close()
		os.Exit(1)
	}

	if err = c.SetCommands([]gotdbot.BotCommand{
		{Command: "start", Description: "Open the bot"},
		{Command: "help", Description: "Backup and restore instructions"},
		{Command: "mongo", Description: "Back up or restore MongoDB"},
		{Command: "ping", Description: "Bot latency and uptime"},
		{Command: "privacy", Description: "Data handling"},
	}, nil); err != nil {
		c.Logger.Warn("Could not register bot commands")
	}

	c.Idle()
}

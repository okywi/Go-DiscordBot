package main

import (
	"GoBot/internal/bot"
	"GoBot/internal/config"
	"log"
)

func main() {
	// load env variables
	config := config.New()

	// add line numbers and file to logger
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// start bot
	bot.Start(&config)
}

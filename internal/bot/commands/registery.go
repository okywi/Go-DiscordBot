package commands

import (
	"GoBot/internal/config"
	"log"

	"github.com/bwmarrin/discordgo"
)

func Register(bot *discordgo.Session, config *config.Config) func() {
	minecraft := newMinecraft(config.SocketPassword, config.ServerIp)
	minecraft.register(bot)
	minecraft.createWebhook(bot)
	tom := newTom(minecraft.ChannelID, config.GeminiApiKey)
	tom.register(bot)

	colorSystem := newColorSystem()
	colorSystem.register(bot)

	kokCounter := newKokCounter()
	kokCounter.register(bot)

	stock := newStock()
	stock.register(bot)

	timers := newTimers(&tom)
	timers.register(bot)

	reactions := newReactions()
	reactions.register(bot)

	// cleanup
	return func() {
		wErr := bot.WebhookDelete(minecraft.Webhook.ID)
		if wErr != nil {
			log.Println("Failed to delete webhook: ", wErr)
		}
		log.Println("Cleaned up successfully.")
	}
}

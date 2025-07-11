package commands

import (
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type reactions struct {
	namesToRemove []string
}

func newReactions() reactions {
	return reactions{
		namesToRemove: []string{"windows", "XP", "bluescreen"},
	}
}

func (reactions *reactions) register(bot *discordgo.Session) {
	// add handlers
	bot.AddHandler(reactions.removeListener)
}

func (reactions *reactions) removeListener(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	emojiName := r.Emoji.Name;

	for _, name := range reactions.namesToRemove {
		if strings.Contains(strings.ToLower(emojiName), name) {
			err := s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.APIName(), r.UserID)
			if err != nil {
				log.Println(err)
			} else {
				log.Println("Removed: ", name)
			}
		}
	}
}
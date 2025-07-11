package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type kokCounter struct {
	FilePath    string
	CountByUser map[string]int
}

func newKokCounter() kokCounter {
	counter := kokCounter{
		FilePath: "assets/data/koks.json",
	}

	counter.CountByUser = counter.read()
	return counter
}

func (counter *kokCounter) register(bot *discordgo.Session) {
	// add handlers
	bot.AddHandler(counter.listener)
	bot.AddHandler(counter.deletionListener)
	bot.AddHandler(counter.kokCountCommand)
}

func (counter *kokCounter) write() {
	counterJson, err := json.MarshalIndent(counter.CountByUser, "", "  ")

	if err != nil {
		log.Println("An error occured while creating the counter json: ", err)
	}

	errWrite := os.WriteFile(counter.FilePath, counterJson, 0666)

	if errWrite != nil {
		log.Println("An error occured while writing counter json to file: ", errWrite)
	}
}

func (counter kokCounter) read() map[string]int {
	counterData, err := os.ReadFile(counter.FilePath)

	if err != nil {
		log.Println("An error occured while reading counter json from file: ", err)
	}

	counters := map[string]int{}

	jErr := json.Unmarshal(counterData, &counters)
	if jErr != nil {
		log.Fatalln("Failed to unmarshal json: ", jErr)
	}
	return counters
}

func getKokAmount(content string) int {
	return strings.Count(content, "<a:kok:1324540733222289490>")
}

func (counter *kokCounter) listener(s *discordgo.Session, m *discordgo.MessageCreate) {
	if !m.Author.Bot {
		kokAmount := getKokAmount(m.Content)
		if kokAmount > 0 {
			counter.CountByUser[m.Author.ID] += kokAmount

			counter.write()
		}
	}
}

func (counter *kokCounter) deletionListener(s *discordgo.Session, message *discordgo.MessageDelete) {
	m := message.BeforeDelete

	if m == nil {
		return
	}

	if !m.Author.Bot {
		kokAmount := getKokAmount(m.Content)
		if kokAmount > 0 {
			count, exists := counter.CountByUser[m.Author.ID]

			if exists {
				counter.CountByUser[m.Author.ID] -= kokAmount
				if count < 0 {
					counter.CountByUser[m.Author.ID] = 0
				}
			}

			counter.write()
		}
	}
}

func (counter *kokCounter) kokCountCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name != "kokcount" {
		return
	}

	userID := i.Member.User.ID

	if i.ApplicationCommandData().Options != nil {
		userID = i.ApplicationCommandData().Options[0].UserValue(s).ID
	}

	member, _ := s.State.Member(i.GuildID, userID)

	response := ""

	if count, exists := counter.CountByUser[member.User.ID]; exists {
		response = fmt.Sprintf("%s hat bereits %d <a:kok:1324540733222289490>'s geschickt", member.DisplayName(), count)
	} else {
		response = fmt.Sprintf("%s hat noch keine <a:kok:1324540733222289490>'s geschickt", member.DisplayName())
	}

	rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
		},
	})
	if rErr != nil {
		log.Println("Failed to send interaction response: ", rErr)
	}

}

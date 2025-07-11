package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
	"google.golang.org/genai"
)

type timers struct {
	timersPath string
	timersData map[string][]timer
	Tom        *genAi
}

type timer struct {
	Id        string
	Date      string
	Message   string
	ChannelId string
	User      string
	Pronouns  string
	GuildId   string
}

func newTimers(tom *genAi) timers {
	return timers{
		timersPath: "assets/data/timers.json",
		Tom:        tom,
	}
}

func (t *timer) create(bot *discordgo.Session, timers *timers) {
	// wait until time is reached
	loc, tErr := time.LoadLocation("Europe/Berlin")
	if tErr != nil {
		log.Println("Failed to load timezone: ", tErr)
	}

	until, err := time.ParseInLocation(time.RFC3339, t.Date, loc)

	if err != nil {
		log.Println("Couldn't parse time for timer: ", err)
	}
	log.Println("time: ", time.Until(until))
	time.Sleep(time.Until(until))

	// add timer message to contents
	timers.Tom.contents = append(timers.Tom.contents, &genai.Content{
		Parts: []*genai.Part{
			{
				Text: fmt.Sprintf("%s %s: %s", t.User, t.Pronouns, t.Message),
			},
		},
		Role: "user",
	})

	// generate ai message
	content, cErr := timers.Tom.client.Models.GenerateContent(context.Background(), timers.Tom.ModelName, timers.Tom.contents, timers.Tom.Config)

	if cErr != nil {
		log.Println("Failed to generate timer ai message: ", cErr)
	}

	response := content.Candidates[0].Content.Parts[0].Text

	// add ai response to contents
	// add timer message to contents
	timers.Tom.contents = append(timers.Tom.contents, &genai.Content{
		Parts: []*genai.Part{
			{
				Text: response,
			},
		},
		Role: "model",
	})

	// send message
	_, mErr := bot.ChannelMessageSend(t.ChannelId, response)
	if mErr != nil {
		log.Println("Failed to send message: ", mErr)
	}

	// remove timer
	timers.timersData[t.GuildId] = slices.DeleteFunc(timers.timersData[t.GuildId], func(timer timer) bool {
		return timer.Id == t.Id
	})

	timers.write()
}

func (timers *timers) register(bot *discordgo.Session) {
	bot.AddHandler(timers.timerCommand)
	bot.AddHandler(timers.load)

	// load timers
	timers.read()

}

func (timers *timers) load(s *discordgo.Session, g *discordgo.GuildCreate) {
	for _, timer := range timers.timersData[g.Guild.ID] {
		go timer.create(s, timers)
	}
}

func (timers *timers) read() {
	// check if file exists
	if _, err := os.Stat(timers.timersPath); err != nil {
		timers.timersData = map[string][]timer{}
		return
	}

	// read file
	if file, fErr := os.ReadFile(timers.timersPath); fErr == nil {
		err := json.Unmarshal(file, &timers.timersData)
		if err != nil {
			log.Fatalln("Couldn't unmarshal timer json: ", err)
			return
		}
	} else {
		log.Println("Couldn't read timer file: ", fErr)
	}
}

func (timers *timers) write() {
	if data, jErr := json.MarshalIndent(timers.timersData, "", "  "); jErr == nil {
		err := os.WriteFile(timers.timersPath, data, 0666)
		if err != nil {
			log.Fatalln("Couldn't write timer file: ", err)
		}
	} else {
		log.Println("Couldn't marshal timer json: ", jErr)
	}
}

func (timers *timers) checkTime(givenTime time.Time, now time.Time) string {
	response := ""

	if now.Compare(givenTime) == 1 {
		response = "That time is in the past."
	} else if now.Compare(givenTime) == 0 {
		response = "That time is now."
	}
	return response
}

func (timers *timers) timerCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	if data.Name != "timer" {
		return
	}

	if data.Options == nil {
		rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Please provide an option.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if rErr != nil {
			log.Println("Failed to send interaction response: ", rErr)
		}
		return
	}

	// create timer
	var response = "The bot will answer on your set time."
	loc, tErr := time.LoadLocation("Europe/Berlin")
	if tErr != nil {
		log.Println("Failed to load timezone: ", tErr)
	}
	var date time.Time
	now := time.Now().In(loc)
	inputTime := now
	message := ""

	for _, option := range data.Options {
		switch option.Name {
		case "message":
			message = option.StringValue()
		case "seconds":
			if value, err := time.ParseDuration(fmt.Sprintf("%ds", option.IntValue())); err == nil {
				inputTime = inputTime.Add(value)
				check := timers.checkTime(inputTime, now)
				if check == "" {
					date = inputTime
				} else {
					response = check
				}

			} else {
				response = "Please only input the number of seconds."
			}
		case "minutes":
			if value, err := time.ParseDuration(fmt.Sprintf("%dm", option.IntValue())); err == nil {
				inputTime = inputTime.Add(value)
				check := timers.checkTime(inputTime, now)
				if check == "" {
					date = inputTime
				} else {
					response = check
				}
			} else {
				response = "Please only input the number of minutes."
			}
		case "hours":
			if value, err := time.ParseDuration(fmt.Sprintf("%dh", option.IntValue())); err == nil {
				inputTime = inputTime.Add(value)
				check := timers.checkTime(inputTime, now)
				if check == "" {
					date = inputTime
				} else {
					response = check
				}
			} else {
				response = "Please only input the number of hours."
			}
		case "time":
			if value, err := time.ParseInLocation(time.TimeOnly, option.StringValue(), loc); err == nil {
				value = value.AddDate(now.Year(), int(now.Month())-1, now.Day()-1)
				check := timers.checkTime(value, now)
				if check == "" {
					date = value
				} else {
					response = check
				}
			} else {
				response = "Please input the time in the correct format: HH:mm:ss"
			}
		case "date":
			if value, err := time.ParseInLocation(time.DateOnly, option.StringValue(), loc); err == nil {
				check := timers.checkTime(value, now)
				if check == "" {
					date = value
				} else {
					response = check
				}
			} else {
				response = "Please input the time in the correct format: Year-Month-Day"
			}
		case "dateandtime":
			if value, err := time.ParseInLocation(time.DateTime, option.StringValue(), loc); err == nil {
				check := timers.checkTime(value, now)
				if check == "" {
					date = value
				} else {
					response = check
				}
			} else {
				response = "Please input the time in the correct format: Year-Month-Day HH:mm:ss"
			}
		}
	}

	guild, _ := s.State.Guild(i.GuildID)

	if !date.IsZero() {
		t := timer{
			Id:        i.ID,
			Date:      date.Format(time.RFC3339),
			Message:   message,
			User:      i.Member.DisplayName(),
			Pronouns:  timers.Tom.getPronouns(guild, i.Member),
			ChannelId: i.ChannelID,
			GuildId:   i.GuildID,
		}

		go t.create(s, timers)

		if _, exists := timers.timersData[i.GuildID]; !exists {
			timers.timersData[i.GuildID] = []timer{}
		}

		timers.timersData[i.GuildID] = append(timers.timersData[i.GuildID], t)

		timers.write()
	}

	// send response
	rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if rErr != nil {
		log.Println("Failed to send interaction response: ", rErr)
	}
}

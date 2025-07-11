package bot

import (
	"GoBot/internal/bot/commands"
	"GoBot/internal/config"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func Start(config *config.Config) {
	// create bot
	bot, err := discordgo.New("Bot " + config.Token)

	if err != nil {
		log.Println("error creating Discord session,", err)
		return
	}

	// Bot settings
	bot.State.MaxMessageCount = 1000
	bot.StateEnabled = true
	bot.Identify.Intents = discordgo.IntentsAll
	bot.ShouldReconnectOnError = true
	bot.Client.Timeout = 0

	// register commands
	cleanup := commands.Register(bot, config)

	// register appCommands
	bot.AddHandler(initializeCommands)

	// close bot after everything is cleaned up
	defer func() {
		err := bot.Close()
		if err != nil {
			log.Fatalln("Failed to close bot: ", err)
		}
		log.Println("Closed bot successfully.")
	}()

	// register things that close when the bot ends
	defer cleanup()

	// start bot
	err = bot.Open()

	if err != nil {
		log.Println("Error opening connection,", err)
		return
	}

	log.Println("Bot is now running. Press CTRL + C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func initializeCommands(s *discordgo.Session, g *discordgo.GuildCreate) {
	for _, command := range appCommands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, g.ID, command)

		if err != nil {
			log.Fatalf("Could not create command (%s): %s", command.Name, err)
		}
	}
}

var appCommands []*discordgo.ApplicationCommand = []*discordgo.ApplicationCommand{
	{
		Name:        "refreshai",
		Description: "Refreshes the system prompt of the ai",
	},
	{
		Name:        "timer",
		Description: "The bot will answer the message after the requested time.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "inform the bot:",
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "seconds",
				Description: "seconds to wait:",
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "minutes",
				Description: "minutes to wait:",
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "hours",
				Description: "hours to wait:",
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "time",
				Description: "send message on specific time at todays day. Format: HH:mm:ss",
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "date",
				Description: "send message at given date at 0a.m. Format: Year-Month-Day",
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "dateandtime",
				Description: "send message at given date and time. Format: Year-Month-Day HH:mm:ss",
			},
		},
	},
	{
		Name:        "kokcount",
		Description: "Outputs how many times you have sent :kok:.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionMentionable,
				Name:        "user",
				Description: "user:",
			},
		},
	},
	{
		Name:        "updatecolor",
		Description: "Creates, updates or removes your color role",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "color",
				Description: "the hex color value the role should have",
				Required:    false,
			},
		},
	},
	{
		Name:        "setcolororderrole",
		Description: "The color roles will be added below this role. Set this to the desired role.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "role",
				Description: "The order role",
				Required:    true,
			},
		},
	},
	{
		Name:        "setcolororderrole",
		Description: "The color roles will be added below this role. Set this to the desired role.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "role",
				Description: "The order role",
				Required:    true,
			},
		},
	},
	{
		Name:        "currentplayers",
		Description: "Outputs the current players of the minecraft server.",
	},
	{
		Name:        "mcreconnect",
		Description: "Reconnect to the minecraft server.",
	},
	{
		Name:        "rheinmetall",
		Description: "Show rheinmetall stock information.",
	},
	{
		Name:        "stock",
		Description: "Show values of the specified stock",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "stock",
				Description: "The stock you want to query",
				Required:    true,
			},
		},
	},
}

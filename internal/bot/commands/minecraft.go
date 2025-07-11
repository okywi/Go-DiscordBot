package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Tnze/go-mc/bot"
	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
)

type minecraft struct {
	conn             *websocket.Conn
	ip               string
	websocketAddress string
	ChannelID        string
	guildID          string
	isConnected      bool
	Webhook          *discordgo.Webhook
	socketPassword   string
}

func newMinecraft(socketPassword string, serverIp string) minecraft {
	// create minecraft bridge
	return minecraft{
		ip:               serverIp,
		websocketAddress: fmt.Sprintf("ws://%s:9459", serverIp),
		ChannelID:        "1349665912898322442",
		guildID:          "1323715581677011067",
		socketPassword:   socketPassword,
	}
}

func (mc *minecraft) createWebhook(bot *discordgo.Session) {
	var err error
	mc.Webhook, err = bot.WebhookCreate(mc.ChannelID, "Bridge", "")

	if err != nil {
		log.Println("Error creating webhook: ", err)
	}
}

func (mc *minecraft) register(bot *discordgo.Session) {
	// add handlers
	bot.AddHandler(mc.createListener)
	bot.AddHandler(mc.playerCountCommand)
	bot.AddHandler(mc.discordMessageListener)
	bot.AddHandler(mc.reconnectCommand)
	bot.AddHandler(mc.channelEditorListener)
}

func (mc minecraft) getPlayerData() (float64, float64, []string) {
	data, _, err := bot.PingAndList(mc.ip)

	if err != nil {
		log.Println("Failed to query server.")
	}

	// Unmarshal into a map
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("Error unmarshaling JSON: %v", err)
		return -1, -1, []string{"Error querying server."}
	}

	playersData := result["players"].(map[string]interface{})

	var maxPlayers float64
	var playerCount float64
	var players []string

	for key, value := range playersData {
		if key == "online" {
			playerCount = value.(float64)
		}
		if key == "max" {
			maxPlayers = value.(float64)
		}
		if key == "sample" {
			for _, value := range value.([]interface{}) {
				for key, value := range value.(map[string]interface{}) {
					if key == "name" {
						players = append(players, value.(string))
					}
				}
			}
		}
	}

	return maxPlayers, playerCount, players
}

func (mc minecraft) playerCountCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {

	interactionData := i.ApplicationCommandData()

	if interactionData.Name != "currentplayers" {
		return
	}

	rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if rErr != nil {
		log.Println("Failed to send interaction response: ", rErr)
	}

	var formattedPlayers string

	maxPlayers, playerCount, players := mc.getPlayerData()

	for _, player := range players {
		formattedPlayers += player + "\n"
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:  "Players:",
			Value: formattedPlayers,
		},
	}

	embed := &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeArticle,
		Title:       fmt.Sprintf("- Current Players (%v/%v) -", playerCount, maxPlayers),
		Description: fmt.Sprintf("There are %v out of %v players online.", playerCount, maxPlayers),
		Color:       convertHexColorToInt("F4B8E4"),
	}
	if playerCount > 0 {
		embed.Fields = fields
	}

	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{
			embed,
		},
	})

	if err != nil {
		log.Println("Error responding: ", err)
	}
}

func (mc *minecraft) createConnection() (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout:  10 * time.Second, // Timeout for the handshake
		EnableCompression: false,            // Disable compression
	}

	// Add custom headers to the handshake
	headers := http.Header{}
	headers.Add("X-Auth-Token", mc.socketPassword)   // Example: Add an auth token
	headers.Add("User-Agent", "Go-WebSocket-Client") // Example: Add a User-Agent

	conn, _, err := dialer.Dial(mc.websocketAddress, headers)

	if err != nil {
		log.Println("Error opening websocket connection: ", err)
		return nil, err
	}

	mc.isConnected = true

	wErr := conn.WriteMessage(websocket.PongMessage, []byte("Pong!"))

	if wErr != nil {
		log.Println("Error sending pong to server.")
	}

	return conn, err
}

func (mc *minecraft) socketListener(s *discordgo.Session, guildID string) {
	if guildID != mc.guildID {
		return
	}

	for {
		if mc.isConnected {
			_, msg, err := mc.conn.ReadMessage()

			if !mc.isConnected {
				return
			}

			if err != nil {
				if websocket.IsCloseError(err) {
					log.Println("Connection closed gracefully.")
				} else if websocket.IsUnexpectedCloseError(err) {
					log.Println("Connection closed unexpectedly.")
				} else {
					log.Println("Error reading message:", err)
				}
				mc.reconnect(s, guildID)
				return
			} else {
				content := string(msg)

				var checkedEmojis []string

				if strings.HasPrefix(content, "MC:") {
					guild, _ := s.State.Guild(guildID)

					// convert emojis
					for _, emoji := range guild.Emojis {
						if !slices.Contains(checkedEmojis, strings.ToLower(emoji.Name)) {
							content = strings.ReplaceAll(content, ":"+strings.ToLower(emoji.Name)+":", emoji.MessageFormat())

							checkedEmojis = append(checkedEmojis, strings.ToLower(emoji.Name))
						}
					}

					// convert mentions
					for _, member := range guild.Members {
						content = strings.ReplaceAll(content, "@"+member.User.Username, "<@"+member.User.ID+">")
					}
					var stickerURLs []string

					// convert stickers
					for _, sticker := range guild.Stickers {
						count := strings.Count(content, sticker.Name)

						if count > 0 {
							if sticker.Available {
								content = strings.ReplaceAll(content, sticker.Name, "")
								var extension string
								switch sticker.FormatType {
								case discordgo.StickerFormatTypeAPNG:
									extension = ".gif"
								case discordgo.StickerFormatTypePNG:
									extension = ".png"
								case discordgo.StickerFormatTypeGIF:
									extension = ".gif"
								}
								stickerURLs = append(stickerURLs, fmt.Sprintf("https://media.discordapp.net/stickers/%s"+extension, sticker.ID))
							}
						}
					}

					// generate sticker files
					var stickers []*discordgo.File

					for _, url := range stickerURLs {
						resp, err := http.Get(url)
						if err != nil {
							continue
						}
						stickers = append(stickers, &discordgo.File{
							Name:   url,
							Reader: resp.Body,
						})
					}

					message := []rune(string(content))
					// remove MC:
					message = message[3:]

					// get name
					name := mc.findNameFromMinecraft(string(message))

					if name != "" {
						message = message[len(name)+2:]
					}

					_, err := s.WebhookExecute(mc.Webhook.ID, mc.Webhook.Token, true, &discordgo.WebhookParams{
						Content:  string(message),
						Username: name,
						Files:    stickers,
					})

					if err != nil {
						log.Println("Failed to send webhook message: ", err)
					}
				}
			}
		}
	}
}

func (mc *minecraft) channelEditorListener(s *discordgo.Session, g *discordgo.GuildCreate) {
	if g.ID == mc.guildID {
		for {
			_, playerCount, players := mc.getPlayerData()

			formattedPlayers := ""

			for i, player := range players {
				seperator := ", "
				if i == len(players)-1 {
					seperator = ""
				}
				formattedPlayers += player + seperator
			}

			var playerCountText string

			switch playerCount {
			case -1:
				playerCountText = ""
			case 0:
				formattedPlayers = "No players online."
				playerCountText = fmt.Sprint("-", playerCount)
			default:
				playerCountText = fmt.Sprint("-", playerCount)
			}

			_, err := s.ChannelEditComplex(mc.ChannelID, &discordgo.ChannelEdit{
				Name:  "🪓minecraft-chat" + playerCountText,
				Topic: formattedPlayers,
			})

			if err != nil {
				log.Println("Can't update channel: ", err)
			}

			time.Sleep(60 * time.Second)
		}
	}
}

func (mc *minecraft) createListener(s *discordgo.Session, g *discordgo.GuildCreate) {
	if g.ID == mc.guildID {
		mc.reconnect(s, g.ID)
	}
}

func (mc *minecraft) convertEmojisFromDiscord(message string) string {
	// remove id from emojis
	emojiRegex := regexp.MustCompile(`(<:|<a:)(\w{2,}):(\d{18,19})>`)

	message = emojiRegex.ReplaceAllStringFunc(message, func(match string) string {
		return strings.ToLower(":" + strings.Split(match, ":")[1] + ":")
	})

	return message
}

func (mc *minecraft) convertMentionsFromDiscord(message string, s *discordgo.Session, guildId string, mentions []*discordgo.User) string {
	// translate mentions to the name <@404992088384471041>
	i := 0
	mentionRegex := regexp.MustCompile(`<@(\d{18,})>`)
	message = mentionRegex.ReplaceAllStringFunc(message, func(text string) string {
		member, _ := s.State.Member(guildId, mentions[i].ID)
		return "@" + member.DisplayName()
	})

	return message
}

func (mc *minecraft) convertAttachmentsFromDiscord(messageAttachments []*discordgo.MessageAttachment) string {
	// check for attachment
	attachments := ""
	if len(messageAttachments) > 0 {
		for i, att := range messageAttachments {
			if i+1 == len(messageAttachments) {
				if i == 0 {
					attachments += "[" + att.Filename + "]"
				} else {
					attachments += att.Filename + "]"
				}
			} else {
				attachments += att.Filename + ", "
			}
		}
		attachments += " "
	}

	return attachments
}

func (mc *minecraft) convertStickersFromDiscord(messageStickers []*discordgo.StickerItem) string {
	// check for stickers
	stickers := ""
	if len(messageStickers) > 0 {
		for i, sticker := range messageStickers {
			if i+1 == len(messageStickers) {
				stickers += sticker.Name
			} else {
				stickers += sticker.Name + ", "
			}
		}
		stickers += " "
	}

	return stickers
}

func (mc *minecraft) findNameFromMinecraft(message string) string {
	if strings.HasPrefix(message, "<") {
		regex := regexp.MustCompile(`<([^>@]+)>`)
		name := regex.FindString(message)

		name = name[1 : len(name)-1]
		return name
	}

	return ""
}

func (mc *minecraft) discordMessageListener(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.GuildID != mc.guildID || m.ChannelID != mc.ChannelID || m.Author.Bot {
		return
	}

	if mc.isConnected {
		member, err := s.State.Member(m.GuildID, m.Author.ID)

		if err != nil {
			log.Println("Error getting Member: ", err)
		}

		// format message
		content := mc.convertEmojisFromDiscord(m.Content)
		content = mc.convertMentionsFromDiscord(content, s, m.GuildID, m.Mentions)
		attachments := mc.convertAttachmentsFromDiscord(m.Attachments)
		stickers := mc.convertStickersFromDiscord(m.StickerItems)

		message := "<" + member.DisplayName() + "> " + attachments + stickers + content

		if m.ReferencedMessage != nil {
			// format message
			refContent := mc.convertEmojisFromDiscord(m.ReferencedMessage.Content)
			refContent = mc.convertMentionsFromDiscord(refContent, s, m.ReferencedMessage.GuildID, m.ReferencedMessage.Mentions)
			attachments := mc.convertAttachmentsFromDiscord(m.ReferencedMessage.Attachments)
			stickers := mc.convertStickersFromDiscord(m.ReferencedMessage.StickerItems)

			// get name
			var name string

			refMember, err := s.State.Member(m.GuildID, m.ReferencedMessage.Author.ID)

			if err != nil {
				//log.Println("Error getting refMember: ", err)
				name = m.ReferencedMessage.Author.Username
			} else {
				name = refMember.DisplayName()

				if m.ReferencedMessage.Author.Bot {
					name = m.ReferencedMessage.Author.Username
				}

				if m.ReferencedMessage.Author.ID == s.State.User.ID {
					mc.findNameFromMinecraft(refContent)
				}

			}
			refMessage := fmt.Sprintf("DC:§7Replying to %s \"§o%s %s %s§r§7\":", name, refContent, attachments, stickers)
			mErr := mc.conn.WriteMessage(websocket.TextMessage, []byte(refMessage))
			if mErr != nil {
				log.Println("Failed to write referencedMessage to connection: ", mErr)
			}
		}

		message = "DC:§7" + message

		mErr := mc.conn.WriteMessage(websocket.TextMessage, []byte(message))

		if mErr != nil {
			log.Println("Failed to write message to connection: ", mErr)
		}
	}
}

func (mc *minecraft) reconnectCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	if data.Name != "mcreconnect" {
		return
	}

	content := mc.reconnect(s, i.GuildID)

	rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if rErr != nil {
		log.Println("Failed to send interaction response: ", rErr)
	}
}

func (mc *minecraft) reconnect(s *discordgo.Session, guildID string) string {
	var content string

	// Close the existing connection if it exists
	if mc.isConnected && mc.conn != nil {
		// Send a close message to the server
		err := mc.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			fmt.Printf("Error sending close message: %v\n", err)
		}
		mc.isConnected = false
		cErr := mc.conn.Close()
		if cErr != nil {
			log.Println("Error closing connection: ", cErr)
			mc.isConnected = true
		} else {
			log.Println("Closed connection.")
		}
	}

	log.Println("Restarting connection.")

	var err error
	mc.conn, err = mc.createConnection()

	if err != nil {
		content = "Error happened. (the websocket is probably not running)"
		log.Println("Retrying in 60 seconds...")
		time.Sleep(60 * time.Second)
		mc.reconnect(s, guildID)
		return content
	} else {
		content = "Successfully reconnected."
		log.Println("Successfully restarted the connection.")
	}

	go mc.socketListener(s, guildID)

	return content
}

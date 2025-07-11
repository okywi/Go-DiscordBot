package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func newColorSystem() colorSystem {
	colorSystem := colorSystem{
		roleByGuildByUsers: map[string]map[string][]string{},
		orderRoleByGuild:   map[string]string{},
		filePathColorRoles: "assets/data/colorRoles.json",
		filePathOrderRole:  "assets/data/orderRole.json",
	}

	colorSystem.read()
	return colorSystem
}

func (colorSystem *colorSystem) register(bot *discordgo.Session) {
	// add handlers
	bot.AddHandler(colorSystem.createRole)
	bot.AddHandler(colorSystem.setOrderRole)
	bot.AddHandler(colorSystem.onMemberRoleDelete)
}

type colorSystem struct {
	roleByGuildByUsers map[string]map[string][]string //guildID [roleID [users]]
	orderRoleByGuild   map[string]string
	filePathColorRoles string
	filePathOrderRole  string
}

func (colorSystem colorSystem) write() {
	orderRoles, err := json.MarshalIndent(colorSystem.orderRoleByGuild, " ", "  ")

	if err != nil {
		log.Fatal("Error marshalling color role data: ", err)
	}

	errWriteOrders := os.WriteFile(colorSystem.filePathOrderRole, orderRoles, 0666)

	if errWriteOrders != nil {
		log.Fatal("Error writing color roles to file: ", errWriteOrders)
	}

	colorRoles, err := json.MarshalIndent(colorSystem.roleByGuildByUsers, " ", "  ")

	if err != nil {
		log.Fatal("Error marshalling color role data: ", err)
	}

	errWriteColors := os.WriteFile(colorSystem.filePathColorRoles, colorRoles, 0666)

	if errWriteColors != nil {
		log.Fatal("Error writing color roles to file: ", errWriteColors)
	}
}

func (colorSystem colorSystem) read() {
	// orderRole.json
	orderRoles, errRead := os.ReadFile(colorSystem.filePathOrderRole)

	if errRead != nil {
		log.Println("Error reading color roles from file: ", errRead)
	}

	errOrder := json.Unmarshal(orderRoles, &colorSystem.orderRoleByGuild)

	if errOrder != nil {
		log.Println("Error unmarshalling color role data: ", errOrder)
	}

	// colorRoles.json
	colorRoles, errRead := os.ReadFile(colorSystem.filePathColorRoles)

	if errRead != nil {
		log.Println("Error reading color roles from file: ", errRead)
	}

	errColor := json.Unmarshal(colorRoles, &colorSystem.roleByGuildByUsers)

	if errColor != nil {
		log.Println("Error unmarshalling color role data: ", errColor)
	}
}

func (colorSystem colorSystem) setOrderRole(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	if data.Name != "setcolororderrole" {
		return
	}

	// check if all options are filled out
	if data.Options == nil {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Please fill out all options.",
			},
		})
		if err != nil {
			log.Println("Failed to send response: ", err)
		}
		return
	}

	role := data.Options[0].RoleValue(s, i.GuildID)

	colorSystem.orderRoleByGuild[i.GuildID] = role.ID

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Successfully set the order role to @%s", role.Name),
		},
	})
	if err != nil {
		log.Println("Failed to send response: ", err)
	}

	colorSystem.write()
}

func (colorSystem colorSystem) onMemberRoleDelete(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if slices.Compare(m.Roles, m.BeforeUpdate.Roles) == -1 {
		guildData := colorSystem.roleByGuildByUsers[m.GuildID]
		for roleID := range guildData {
			// check if a color role was removed
			if slices.Contains(m.BeforeUpdate.Roles, roleID) && !slices.Contains(m.Roles, roleID) {
				if len(guildData[roleID]) > 1 {
					for _, userID := range guildData[roleID] {
						colorSystem.roleByGuildByUsers[m.GuildID][roleID] = slices.DeleteFunc(guildData[roleID], func(id string) bool {
							return id == userID
						})
					}
				} else {
					delete(colorSystem.roleByGuildByUsers[m.GuildID], roleID)
				}
			}
		}
		return
	}

	colorSystem.write()
}

func (colorSystem colorSystem) removeRole(s *discordgo.Session, guildID string, memberID string, excludeRoleID string) bool {
	removedRole := false
	// check if old role needs to be removed
	guildData := colorSystem.roleByGuildByUsers[guildID]
	for roleID := range guildData {
		for index, userID := range guildData[roleID] {
			if userID == memberID && roleID != excludeRoleID {
				if len(guildData[roleID]) > 1 {
					colorSystem.roleByGuildByUsers[guildID][roleID] = append(colorSystem.roleByGuildByUsers[guildID][roleID][:index], colorSystem.roleByGuildByUsers[guildID][roleID][index+1:]...)

					err := s.GuildMemberRoleRemove(guildID, userID, roleID)

					if err != nil {
						log.Println("Error occured when removing role of member: ", err)
					}

					removedRole = true
				} else {
					delete(colorSystem.roleByGuildByUsers[guildID], roleID)

					err := s.GuildRoleDelete(guildID, roleID)
					if err != nil {
						log.Println("Error occured when deleting role: ", err)
					}

					removedRole = true
				}
			}
		}
	}

	return removedRole
}

func convertHexColorToInt(color string) int {
	colorAsDecimal, colorErr := strconv.ParseInt(color, 16, 32)

	if colorErr != nil {
		log.Println("Error while converting hex code to int:", colorErr)
		return 0
	}

	return int(colorAsDecimal)
}

func (colorSystem colorSystem) createRole(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	if data.Name != "updatecolor" {
		return
	}

	guild, _ := s.State.Guild(i.GuildID)

	// check if the guild has a order role
	orderRoleID, orderRoleExistsInData := colorSystem.orderRoleByGuild[guild.ID]

	if !orderRoleExistsInData {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You need to create an order role with /setordercolorrole",
			},
		})
		if err != nil {
			log.Println("Failed to send response: ", err)
		}
		return
	}

	orderRole, roleErr := s.State.Role(guild.ID, orderRoleID)

	if roleErr != nil {
		rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Your order role does not exist anymore. Set a new one with /setordercolorrole",
			},
		})
		if rErr != nil {
			log.Println("Failed to send interaction response: ", rErr)
		}
		return
	}

	// check if all options are filled out
	if data.Options == nil {
		haveRemovedRole := colorSystem.removeRole(s, guild.ID, i.Member.User.ID, "")

		content := ""
		if haveRemovedRole {
			content = "Removed role from you."
		} else {
			content = "Can't remove role because you have no color role."
		}

		rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
			},
		})
		if rErr != nil {
			log.Println("Failed to send interaction response: ", rErr)
		}

		return
	}

	color := strings.ToLower(data.Options[0].StringValue())

	// check hex code
	if !validateHexCode(color) {
		rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Please enter a correct hex code.",
			},
		})
		if rErr != nil {
			log.Println("Failed to send interaction response: ", rErr)
		}
		return
	}

	// remove # if given
	if strings.HasPrefix(color, "#") {
		color = color[1:7]
		log.Println(color)
	}

	intColor := convertHexColorToInt(color)

	var newRole *discordgo.Role
	roleAlreadyExists := false
	// check if role with the same color already exists
	for _, role := range guild.Roles {
		if role.Color == intColor {
			if _, exists := colorSystem.roleByGuildByUsers[guild.ID][role.ID]; exists {
				newRole = role
				roleAlreadyExists = true
			}
		}

	}

	if !roleAlreadyExists {
		role, err := s.GuildRoleCreate(i.GuildID, &discordgo.RoleParams{
			Name:  color,
			Color: &intColor,
		})

		// check for errors in role creation
		if err != nil {
			rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error occured when creating role.",
				},
			})
			if rErr != nil {
				log.Println("Failed to send interaction response: ", rErr)
			}
			return
		}

		newRole = role
	}

	// check if guild exists in data
	if _, exists := colorSystem.roleByGuildByUsers[guild.ID]; !exists {
		colorSystem.roleByGuildByUsers[guild.ID] = map[string][]string{}
	}

	// create user or add user to entry
	if !slices.ContainsFunc(colorSystem.roleByGuildByUsers[guild.ID][newRole.ID], func(id string) bool {
		return i.Member.User.ID == id
	}) {
		colorSystem.roleByGuildByUsers[guild.ID][newRole.ID] = append(colorSystem.roleByGuildByUsers[guild.ID][newRole.ID], i.Member.User.ID)
	}

	// update role positions if role does not exist
	if !roleAlreadyExists {
		rolesToPosition := guild.Roles

		for _, role := range rolesToPosition {
			// move every role up thats above order role that is not the new role
			if role.Position < orderRole.Position && role.ID != newRole.ID {
				role.Position -= 1
			}
			// set position of last role to the position below the order role
			if role.ID == newRole.ID {
				role.Position = orderRole.Position - 1
			}
		}

		// reorder guild roles
		_, reorderErr := s.GuildRoleReorder(guild.ID, rolesToPosition)
		if reorderErr != nil {
			log.Print("Failed to reorder roles: ", reorderErr)
			_, err := s.ChannelMessageSend(i.ChannelID, "The role was created but reordering it failed. You will have to manually reorder it.")
			if err != nil {
				log.Println("Failed to send message: ", err)
			}
		}
	}

	colorSystem.removeRole(s, guild.ID, i.Member.User.ID, newRole.ID)

	// add role to member
	err := s.GuildMemberRoleAdd(guild.ID, i.Member.User.ID, newRole.ID)

	if err != nil {
		log.Println("Failed adding role to member: ", err)
	}

	colorSystem.write()

	rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Created and added you to the Role %s", newRole.Name),
		},
	})
	if rErr != nil {
		log.Println("Failed to send interaction response: ", rErr)
	}
}

func validateHexCode(code string) bool {
	isMatch, err := regexp.MatchString("^#?([a-f0-9]{6})$", code)

	if err != nil {
		log.Print("Error occured while validating hex code", err)
	}

	return isMatch
}

package main

import (
	"encoding/json"
	"errors"
	"github.com/bwmarrin/discordgo"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

// Config struct holds the configuration settings for the bot.
type Config struct {
	Token  string `json:"token"`  // Bot token used to authenticate with Discord API.
	Prefix string `json:"prefix"` // Prefix for bot commands.
}

var config Config

func main() {
	// Load configuration from 'config.json' file.
	err := loadConfig("config.json")
	if err != nil {
		log.Println("Error loading configuration:", err.Error())
		return
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Println("Error creating Discord session:", err.Error())
		return
	}

	// Register the messageCreate function as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// Set up the bot to receive message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and start listening.
	err = dg.Open()
	if err != nil {
		log.Println("Error opening connection:", err.Error())
		return
	}

	// Wait here until CTRL-C or other termination signal is received.
	log.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// messageCreate is the callback function to handle incoming messages from Discord.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages created by the bot itself.
	if m.Author.ID == s.State.User.ID {
		return
	}

	log.Println(m.Content)

	// Handle bot commands.
	if strings.HasPrefix(m.Content, config.Prefix) {
		command := strings.TrimPrefix(m.Content, config.Prefix)

		// Handle 'ping' command.
		if command == "ping" {
			_, err := s.ChannelMessageSend(m.ChannelID, "Pong!")
			if err != nil {
				log.Println("Error sending message:", err.Error())
				return
			}
		}

		// Handle 'roll' command.
		if strings.HasPrefix(command, "roll") {
			rollCommand := strings.TrimPrefix(command, "roll ")
			dice, total, err := roll(rollCommand)
			if err != nil {
				log.Println("Error rolling dice:", err.Error())
				return
			}
			var message string
			message = "Rolls: "
			for i := range dice {
				message = message + strconv.Itoa(dice[i]) + " "
			}
			message = message + "\nTotal: " + strconv.Itoa(total)
			_, err = s.ChannelMessageSend(m.ChannelID, message)
			if err != nil {
				log.Println("Error sending message:", err.Error())
				return
			}
		}
	}
}

// loadConfig loads configuration from a JSON file and populates the config variable.
func loadConfig(filePath string) error {
	log.Println("Loading configuration...")
	// Open the config file.
	configFile, err := os.Open(filePath)
	if err != nil {
		return errors.New("error opening config file: " + err.Error())
	}
	defer configFile.Close()

	// Read and unmarshal the JSON data into the config variable.
	byteValue, err := io.ReadAll(configFile)
	if err != nil {
		return errors.New("error reading config file: " + err.Error())
	}

	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		return errors.New("error unmarshalling config data: " + err.Error())
	}

	// For testing the reading of token and prefix.
	log.Println("Config loaded successfully.")
	log.Println("Bot Token:", config.Token)
	log.Println("Command Prefix:", config.Prefix)

	return nil
}

// roll simulates rolling dice based on the provided input.
func roll(input string) ([]int, int, error) {
	components := strings.Split(input, "d")
	if len(components) != 2 {
		return nil, 0, errors.New("invalid roll command format")
	}

	num, err := strconv.Atoi(components[0])
	if err != nil {
		return nil, 0, errors.New("invalid number of dice: " + err.Error())
	}

	sides, err := strconv.Atoi(components[1])
	if err != nil {
		return nil, 0, errors.New("invalid number of sides: " + err.Error())
	}

	var dice []int
	var total int

	for i := 0; i < num; i++ {
		n := rand.Intn(sides) + 1
		dice = append(dice, n)
		total += n
	}

	return dice, total, nil
}

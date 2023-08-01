package main

import (
	"encoding/json"
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

type Config struct {
	Token  string `json:"token"`
	Prefix string `json:"prefix"`
}

var config Config

func main() {
	loadConfig("config.json")

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Println("error creating Discord session,", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	//Ignore messages created by the bot
	if m.Author.ID == s.State.User.ID {
		return
	}

	log.Println(m.Content)

	//Commands
	if strings.HasPrefix(m.Content, config.Prefix) {
		m.Content = strings.TrimPrefix(m.Content, config.Prefix)
		if m.Content == "ping" {
			_, err := s.ChannelMessageSend(m.ChannelID, "Pong!")
			if err != nil {
				log.Println("error sending message: ", err)
				return
			}
		}
		if strings.HasPrefix(m.Content, "roll") {
			var r string
			r = strings.TrimPrefix(m.Content, "roll ")
			a, t := roll(r)
			var message string
			message = "Rolls: "
			for i := range a {
				message = message + strconv.Itoa(a[i]) + " "
			}
			message = message + "\nTotal: " + strconv.Itoa(t)
			_, err := s.ChannelMessageSend(m.ChannelID, message)
			if err != nil {
				log.Println("error sending message: ", err)
				return
			}
		}
	}
}

func loadConfig(dp string) {
	log.Println("Hello World")
	//Open Config
	configFile, err := os.Open(dp)
	//Check for error
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Successfully opened config file")
	//defer closing of file
	defer func(configFile *os.File) {
		err := configFile.Close()
		if err != nil {

		}
	}(configFile)

	//Read in config
	byteValue, _ := io.ReadAll(configFile)

	json.Unmarshal(byteValue, &config)
	//For testing reading of token
	log.Println(config.Token)
	log.Println(config.Prefix)
}

func roll(r string) (a []int, t int) {
	s := strings.Split(r, "d")
	num, _ := strconv.Atoi(s[0])
	sides, _ := strconv.Atoi(s[1])

	for i := 0; i < num; i++ {
		n := rand.Intn(sides) + 1
		a = append(a, n)
		t = t + n
	}
	return
}

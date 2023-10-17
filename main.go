package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	Token  string `json:"token"`
	Prefix string `json:"prefix"`
	DbProt string `json:"dbProt"`
	DbUser string `json:"dbUser"`
	DbPass string `json:"dbPass"`
	DbHost string `json:"dbHost"`
	DbOptn string `json:"dbOptn"`
	DbName string `json:"dbName"`
	DbUri  string
}

type Player struct {
	Id       string
	CharName string
	Gold     int
	Items    []int
}

type Item struct {
	Id   int
	Name string
	Desc string
	Cost string
	Sell string
}

type Store struct {
	Id   int
	Name string
	Inv  []int
}

var config Config
var db *mongo.Client

func main() {
	// Load configuration from 'config.json' file.
	err := loadConfig("config.json")
	if err != nil {
		log.Println("Error loading configuration: ", err.Error())
		return
	}

	// Connect and test DB
	db, err = connectDB(config.DbUri, config.DbName)
	if err != nil {
		log.Println("Error connecting to DB: ", err.Error())
		return
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Println("Error creating Discord session: ", err.Error())
		return
	}

	// Register the messageCreate function as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// Set up the bot to receive message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and start listening.
	err = dg.Open()
	if err != nil {
		log.Println("Error opening connection: ", err.Error())
		return
	}

	// Wait here until CTRL-C or other termination signal is received.
	log.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
	err = db.Disconnect(context.TODO())
	if err != nil {
		return
	}
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

		// Handle 'newplayer' command.
		if strings.HasPrefix(command, "newplayer") {
			// Extract userID and name from the message.
			parts := strings.Split(command, " ")
			if len(parts) != 3 {
				log.Println("Invalid command format. Correct usage: !newplayer <@userID> Name")
				return
			}

			userID := parts[1]
			name := parts[2]

			// Call newPlayer to create a new player in the database.
			err := newPlayer(userID, name, db, config.DbName)
			if err != nil {
				log.Println("Error creating new player:", err.Error())
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

	config.DbUri = config.DbProt + "://" + config.DbUser + ":" + config.DbPass + "@" + config.DbHost + "/?" + config.DbOptn
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

// connectDB initializes and connects to the DB
func connectDB(uri string, dbName string) (*mongo.Client, error) {
	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)

	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	// Send a ping to confirm a successful connection
	var result bson.M
	if err := client.Database(dbName).RunCommand(context.TODO(), bson.D{{"ping", 1}}).Decode(&result); err != nil {
		return nil, err
	}

	log.Println("Pinged your deployment. You successfully connected to MongoDB!")
	return client, nil
}

func updatePlayer(client *mongo.Client, player *Player, dbName string) error {
	collection := client.Database(dbName).Collection("BathtubPlayers")

	// Check if the player with the given ID already exists
	filter := bson.M{"id": player.Id}
	existingPlayer := Player{}
	err := collection.FindOne(context.TODO(), filter).Decode(&existingPlayer)

	if err == nil {
		// Player exists, update the entry
		update := bson.M{"$set": bson.M{"charname": player.CharName, "gold": player.Gold, "items": player.Items}}
		_, err = collection.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			return fmt.Errorf("error updating player: %v", err)
		}
	} else {
		// Player doesn't exist, insert as a new player
		_, err = collection.InsertOne(context.TODO(), player)
		if err != nil {
			return fmt.Errorf("error inserting new player: %v", err)
		}
	}

	return nil
}

func newPlayer(userID, name string, client *mongo.Client, dbName string) error {
	// Create a new Player
	newPlayer := Player{
		Id:       userID,
		CharName: name,
		Gold:     100,
		Items:    []int{},
	}

	// Call updatePlayer to push the new player to the database
	err := updatePlayer(client, &newPlayer, dbName)
	if err != nil {
		return fmt.Errorf("error creating new player: %v", err)
	}

	return nil
}

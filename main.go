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
	Sudoer string `json:"sudoer"`
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
	Cost int
	Sell int
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
	log.Println(m.Author.ID)

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

		//Sudoer Commands
		if m.Author.ID == config.Sudoer {
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

			// Handle 'newitem' command.
			if strings.HasPrefix(command, "newitem") {
				// Example input string format: "<Stick><Just a stick><10><20>"
				// Remove the command prefix and angle brackets
				input := strings.TrimPrefix(command, "newitem ")
				input = strings.Trim(input, "<>")
				parts := strings.Split(input, "><")

				if len(parts) != 4 {
					log.Println("Invalid command format. Correct usage: !newitem <name><desc><cost><sell>")
					return
				}

				// Extract parameters
				itemName := parts[0]
				itemDesc := parts[1]

				// Parse cost and sell values
				itemCost, err := strconv.Atoi(parts[2])
				if err != nil {
					log.Println("Invalid item cost:", err.Error())
					return
				}

				itemSell, err := strconv.Atoi(parts[3])
				if err != nil {
					log.Println("Invalid item sell value:", err.Error())
					return
				}

				// Get the next available item ID.
				nextItemID, err := getNextItemID(db, config.DbName)
				if err != nil {
					log.Println("Error getting next item ID:", err.Error())
					return
				}

				// Call newItem to create a new item in the database.
				err = newItem(nextItemID, itemName, itemDesc, itemCost, itemSell, db, config.DbName)
				if err != nil {
					log.Println("Error creating new item:", err.Error())
					return
				}

				_, err = s.ChannelMessageSend(m.ChannelID, "New item added successfully.")
				if err != nil {
					log.Println("Error sending message:", err.Error())
					return
				}
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

func updateItem(client *mongo.Client, item *Item, dbName string) error {
	collection := client.Database(dbName).Collection("BathtubItems")

	filter := bson.M{"id": item.Id}
	existingItem := Item{}
	err := collection.FindOne(context.TODO(), filter).Decode(&existingItem)

	if err == nil {
		update := bson.M{"$set": bson.M{"name": item.Name, "desc": item.Desc, "cost": item.Cost, "sell": item.Sell}}
		_, err = collection.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			return fmt.Errorf("error updating item: %v", err)
		}
	} else {
		// Player doesn't exist, insert as a new player
		_, err = collection.InsertOne(context.TODO(), item)
		if err != nil {
			return fmt.Errorf("error inserting new itemr: %v", err)
		}
	}
	return nil
}

func newItem(id int, name string, desc string, cost int, sell int, client *mongo.Client, dbName string) error {
	newItem := Item{
		Id:   id,
		Name: name,
		Desc: desc,
		Cost: cost,
		Sell: sell,
	}

	err := updateItem(client, &newItem, dbName)
	if err != nil {
		return fmt.Errorf("error creating new item: %v", err)
	}

	return nil
}

func getNextItemID(client *mongo.Client, dbName string) (int, error) {
	collection := client.Database(dbName).Collection("BathtubItems")

	// Sort the items by ID in descending order and find the maximum.
	opts := options.Find().SetSort(bson.D{{"id", -1}}).SetLimit(1)
	cur, err := collection.Find(context.TODO(), bson.D{}, opts)
	if err != nil {
		return 0, err
	}
	defer cur.Close(context.TODO())

	var item Item
	if cur.Next(context.TODO()) {
		if err := cur.Decode(&item); err != nil {
			return 0, err
		}
		return item.Id + 1, nil
	}

	// If no items found, start with ID 1.
	return 1, nil
}

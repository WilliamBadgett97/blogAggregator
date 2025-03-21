package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/WilliamBadgett97/blogaggregator/internal/config"
	"github.com/WilliamBadgett97/blogaggregator/internal/database"
	_ "github.com/lib/pq"
)



func main() {


	// 1. Read config data first
	configData, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	// 2. Create the state with the config
	state := &config.State{
		Cfg: &configData,
	}

	// 3. Set up the commands
	commands := &config.Commands{
		CommandList: make(map[string]func(*config.State, config.Command) error),
		CommandListDetails: make(map[string]string),
	}

	commands.Register("login", config.HandlerLogin, "Get username from cli args, login and store user into DB, as signed in user! e.g ./blogaggregator login <username> ")
	commands.Register("register", config.HandlerRegister, "Get username from cli args to register user. e.g ./blogaggregator register <username> ")
	commands.Register("reset", config.HandlerDeleteUsers, "Empites Users table from database. e.g ./blogaggregator reset ")
	commands.Register("users", config.HandlerGetAllusers, "Returns list of all users. e.g ./blogaggregator users")
	commands.Register("feeds", config.HanlderGetAllFeeds, "Returns list of all feeds. e.g ./blogaggregator feeds")
	commands.Register("follow", config.MiddlewareLoggedIn(config.HandlerFollow), "Returns list of all users. e.g ./blogaggregator users")
	commands.Register("following", config.MiddlewareLoggedIn(config.HandlerFollowing), "Gets currently signed in user and returns all the feeds the user follows e.g ./blogaggregator following")
	commands.Register("unfollow", config.MiddlewareLoggedIn(config.HandlerUnfollow), "Unfollows the provided url from cli of the current signed in user e.g ./blogaggregator <url> ")
	commands.Register("agg", config.HandlerAggCommand, "Constantly scrapes the signed in users feed, based on the timer provided via cli ./blogaggregator <1s> ")
	commands.Register("addfeed", config.MiddlewareLoggedIn(config.HandlerAddToFeed), "Gets current signed in user, along with a url to create a feed, and follows the feed by the signed in user, e.g ./blogaggregator <url> ")
	commands.Register("browse", config.MiddlewareLoggedIn(config.HandlerBrowse),"Accept a number of max posts to return from the current signed in users feed (Newest first) e.g ./blogaggregator <int>")
	// commands.Register("help",nil,"")


	dbURL := state.Cfg.DBURL
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Unable to connect to db", err)
	}
	dbQueries := database.New(db)
	state.Db = dbQueries



	if len(os.Args) < 2 {
		log.Fatal("Not enough arguments provided.")
	}

	// Create a command from the arguments
	command := config.Command{
		Name: os.Args[1],
		Args: os.Args[2:],
	}

	// Run the command
	err = commands.Run(state, command)
	if err != nil {
		if os.Args[1] == "help"{
			for desc := range commands.CommandListDetails {
				fmt.Println(desc,": ",commands.CommandListDetails[desc])
			}
		} else {
		log.Fatal(err)
		}
	}
}

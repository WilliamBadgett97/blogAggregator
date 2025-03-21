// -- goose postgres postgres://williambadgett:@localhost:5432/gator up
// -- goose postgres "postgres://williambadgett:@localhost:5432/gator" create create_feeds_table sql
package config

// goose -dir sql/schema postgres "postgres://williambadgett:@localhost:5432/gator" down
// goose -dir sql/schema postgres "postgres://williambadgett:@localhost:5432/gator" up

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/WilliamBadgett97/blogaggregator/internal/database"
	"github.com/google/uuid"
)

const configFileName = ".gatorconfig.json"

func (c *Config) SetUser(name string) error {
	c.CurrentUserName = name
	return write(*c)
}

func write(cfg Config) error {
	filePath, err := getConfigFilePath()

	if err != nil {
		return err
	}

	b, err := json.Marshal(cfg)

	if err != nil {
		return err
	}

	err = os.WriteFile(filePath, b, 0644)
	if err != nil {
		return err
	}
	return nil
}

func getConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	filePath := homeDir + "/" + configFileName
	return filePath, nil
}

func Read() (Config, error) {
	// Create a new instance of our config struct
	newConfig := Config{}
	filePath, err := getConfigFilePath()
	if err != nil {
		return Config{}, fmt.Errorf("error getting file path %s", err)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return Config{}, fmt.Errorf("error opening file  %s", err)
	}

	defer file.Close()

	stat, err := file.Stat()

	if err != nil {
		return Config{}, fmt.Errorf("error getting file stats %s", err)
	}

	bytes := make([]byte, stat.Size())

	_, err = file.Read(bytes)

	if err != nil {
		return Config{}, fmt.Errorf("error reading file  %s", err)
	}

	err = json.Unmarshal(bytes, &newConfig)

	if err != nil {
		return Config{}, fmt.Errorf("error unmarshalling json %s", err)
	}
	return newConfig, nil
}

func HandlerLogin(s *State, cmd Command) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("the login handler expects a single argument, the username")
	}
	name := cmd.Args[0]
	ctx := context.Background()
	user, found := s.Db.GetUser(ctx, name)
	if found != nil {
		if strings.Contains(found.Error(), "no rows in result set") {
			log.Fatal("User not registered.")
		}
	}
	err := s.Cfg.SetUser(user.Name)

	if err != nil {
		return fmt.Errorf("something went wrong logging in, %v", err)
	}
	fmt.Print("User successfully signed in!")
	return nil
}

func HandlerDeleteUsers(s *State, cmd Command) error {
	ctx := context.Background()
	err := s.Db.DeleteAllUsers(ctx)
	if err != nil {
		log.Fatal(err.Error())
	}
	return nil
}

func HandlerGetAllusers(s *State, cmd Command) error {
	ctx := context.Background()
	users, err := s.Db.GetAllUsers(ctx)
	if err != nil {
		log.Fatal(err.Error())
	}
	// List all users,
	// if user is current signed in, add ()
	for i := 0; i < len(users); i++ {
		if s.Cfg.CurrentUserName == users[i].Name {
			fmt.Println("* ", users[i].Name+" (current)")
		} else {
			fmt.Println(users[i].Name)
		}
	}
	return nil
}

func HandlerRegister(s *State, cmd Command) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("the register handler expects a single argument, the username")
	}
	name := cmd.Args[0]
	ctx := context.Background()
	params := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      name,
	}

	user, err := s.Db.CreateUser(ctx, params)
	if err != nil {
		// Check if this is a duplicate user error
		if strings.Contains(err.Error(), "duplicate key") {
			// Exit with code 1 as specified in the instructions
			fmt.Println("User with that name already exists")
			os.Exit(1)
		}
		return err
	}

	s.Cfg.SetUser(user.Name)
	fmt.Println("Successfully registered user: ", user)
	return nil
}

func (c *Commands) Register(name string, f func(*State, Command) error,  desc string) {
	c.CommandList[name] = f
	// fmt.Print(c.CommandListDetails)
	c.CommandListDetails[name] = desc
}
func (c *Commands) Run(s *State, cmd Command) error {
	// make sure command is within map
	commandToRun, found := c.CommandList[cmd.Name]

	if !found {
		return fmt.Errorf("command not found")
	}
	err := commandToRun(s, cmd)
	if err != nil {
		return err
	}
	return nil

}

func FetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	var feed RSSFeed
	c := http.Client{Timeout: time.Duration(1) * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)

	if err != nil {
		return &RSSFeed{}, err
	}
	req.Header.Set("User-Agent", "Gator")
	resp, err := c.Do(req)
	if err != nil {
		return &RSSFeed{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &RSSFeed{}, err
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return &RSSFeed{}, err
	}
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := 0; i < len(feed.Channel.Item); i++ {
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
	}
	return &feed, nil
}

func HandlerAddToFeed(s *State, cmd Command, user database.User) error {
	// Validate arguments
	if len(cmd.Args) < 2 {
		return errors.New("usage: addfeed <name> <url>")
	}

	ctx := context.Background()
	
	params := database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.Args[0],
		Url:       cmd.Args[1],
		UserID:    user.ID,
	}

	feed, err := s.Db.CreateFeed(ctx, params)
	if err != nil {
		return err
	}

	paramsFeed := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID: user.ID,
		FeedID: feed.ID,
	}

	feedFollows, err := s.Db.CreateFeedFollow(ctx,paramsFeed)
	if err != nil {
		return err
	}
	fmt.Print(feedFollows)

	// Format the output nicely
	fmt.Printf("Feed created:\n")
	fmt.Printf("  ID: %s\n", feed.ID)
	fmt.Printf("  Name: %s\n", feed.Name)
	fmt.Printf("  URL: %s\n", feed.Url)
	fmt.Printf("  Created: %s\n", feed.CreatedAt)

	return nil
}

func HanlderGetAllFeeds(s *State, cmd Command) error {
	ctx := context.Background()
	feeds, err := s.Db.GetFeedsByUser(ctx)
	if err != nil {
		return err
	}
	for _, feed := range feeds {
		fmt.Printf("Feed Name: %s\nURL: %s\nCreator: %s\n\n",
			feed.FeedName, feed.Url, feed.UserName)
	}
	// fmt.Print(feeds)
	return nil
}

func HandlerFollow(s *State, cmd Command, user database.User) error {
	ctx := context.Background()
	feed, err := s.Db.GetFeedByUrl(ctx, cmd.Args[0])
	if err != nil {
		return err
	}
	params := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID: user.ID,
		FeedID: feed.ID,
	}

	feedFollows, err := s.Db.CreateFeedFollow(ctx,params)
	if err != nil {
		return err
	}
	fmt.Println(feedFollows)

	return nil

}

func HandlerFollowing (s *State, cmd Command, user database.User) error {
	ctx := context.Background()
	feedsByUser, err := s.Db.GetFeedFollowsForUser(ctx,user.ID)
	if err != nil {
		return err
	}
	for _, feed := range feedsByUser {
		fmt.Println("Following: ", feed.FeedName)
	}

return nil
}

func HandlerUnfollow (s *State, cmd Command, user database.User) error {
	ctx := context.Background()
	feed, err := s.Db.GetFeedByUrl(ctx,cmd.Args[0])
	if err != nil {
		return err
	}
	params := database.DeleteFeedFollowsByUserAndUrlParams{
		UserID: user.ID,
		FeedID: feed.ID,
	}
	deletedErr := s.Db.DeleteFeedFollowsByUserAndUrl(ctx,params)
	if deletedErr != nil {
		return err
	}
	fmt.Print("Successfully deleted.")
	return nil
}

func HandlerScrapeFeed(s *State, cmd Command) error {
	ctx := context.Background()
	feed, err := s.Db.GetNextFeedToFetch(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Fetching feed from URL: %s\n", feed.Url)
	currentTime := time.Now()
	params := database.MarkFeedFetchedParams{
		LastFetchedAt: sql.NullTime{
			Time:  currentTime,
			Valid: true, // Set Valid to true because currentTime is not NULL
		},
		ID: feed.ID, // Example feed ID
	}

	feedErr := s.Db.MarkFeedFetched(ctx, params)
	
	if feedErr != nil {
		return err
	}

	fetchedFeed, err := FetchFeed(ctx, feed.Url)
	if err != nil {
		return err
	}

	pubDate := fetchedFeed.Channel.Item[0].PubDate
	parsedTime, err := time.Parse(time.RFC1123Z, pubDate) 
	if err != nil {
		// Handle error - the format might be different
		fmt.Printf("Error parsing time: %v\n", err)
		// You might need to try different formats
	}
	postParams := database.CreatePostParams {
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Title: fetchedFeed.Channel.Title,
		Url: feed.Url,
		Description: fetchedFeed.Channel.Description,
		PublishedAt: parsedTime,
		FeedID: feed.ID,
	}
	post, err := s.Db.CreatePost(ctx,postParams)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
		} else {
			fmt.Print(err.Error())
		}
	}
	fmt.Print(post.Title, "SAVED")

	return nil
}


func HandlerAggCommand(s *State, cmd Command) error {
	// Make sure there's at least one argument
	if len(cmd.Args) < 1 {
		return errors.New("missing time between requests argument")
	}

	timeBetweenPost, err := time.ParseDuration(cmd.Args[0])
	if err != nil {
		return err
	}

	fmt.Println("Collecting Feeds Every", timeBetweenPost)

	ticker := time.NewTicker(timeBetweenPost)
	for ; ; <-ticker.C {
		err := HandlerScrapeFeed(s, cmd)
		if err != nil {
			// Consider if you want to terminate the loop on error
			// or just log the error and continue
			fmt.Printf("Error scraping feed: %v\n", err)
			// To continue despite errors, remove the return statement
			// return err
		}
	}
}

func HandlerBrowse(s *State, cmd Command, user database.User) error {
    var limit int32
    if len(cmd.Args) > 0 {
        parsedValue, err := strconv.ParseInt(cmd.Args[0], 10, 32)
        if err != nil {
            return err 
        }

        limit = int32(parsedValue)
    } else {
		limit = 2
	}

	ctx := context.Background()
	params := database.GetPostsForUserParams{
		UserID: user.ID,
		Limit: int32(limit),
	}
	posts, err := s.Db.GetPostsForUser(ctx, params)
	if err != nil {
		return err
	}
		// Format the output nicely instead of just printing the struct
		if len(posts) == 0 {
			fmt.Println("No posts found.")
			return nil
		}
		
		for _, post := range posts {
			fmt.Printf("Title: %s\n", post.Title)
			fmt.Printf("Published: %s\n", post.PublishedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("URL: %s\n", post.Url)
			fmt.Print("Desc:", post.Description)
			fmt.Println("-----------------------------------")
		}
	return nil
}


func MiddlewareLoggedIn(handler func(s *State, cmd Command, user database.User) error) func(*State, Command) error {
	return func(s *State, c Command) error {
		ctx := context.Background()
		user, err := s.Db.GetUser(ctx, s.Cfg.CurrentUserName)
		if err != nil {
			return err
		}
		 handlerErr := handler(s,c,user)
		 if handlerErr != nil {
			return handlerErr
		 }
		return nil
	}
}



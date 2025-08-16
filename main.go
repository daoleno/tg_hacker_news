package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	BatchSize            = 30
	NumCommentsThreshold = 5
	ScoreThreshold       = 50
	DefaultTimeout       = 9 * time.Minute
	Hot                  = "🔥"
	TelegramAPIBase      = "https://api.telegram.org/"
	HackerNewsAPIBase    = "https://hacker-news.firebaseio.com/v0"
	CleanupInterval      = 24 * time.Hour
	PollInterval         = 5 * time.Minute
)

type Config struct {
	BotKey string
	ChatID string
	DBPath string
}

type Story struct {
	ID          int64     `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Descendants int64     `json:"descendants"`
	Score       int64     `json:"score"`
	Type        string    `json:"type"`
	MessageID   int64     `json:"-"`
	LastSave    time.Time `json:"-"`
}

type SendMessageRequest struct {
	ChatID              string               `json:"chat_id"`
	Text                string               `json:"text"`
	ParseMode           string               `json:"parse_mode,omitempty"`
	ReplyMarkup         InlineKeyboardMarkup `json:"reply_markup,omitempty"`
	DisableNotification bool                 `json:"disable_notification,omitempty"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard,omitempty"`
}

type InlineKeyboardButton struct {
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

type SendMessageResponse struct {
	OK     bool   `json:"ok"`
	Result Result `json:"result"`
}

type Result struct {
	MessageID int64 `json:"message_id"`
}

type EditMessageTextRequest struct {
	ChatID      string               `json:"chat_id"`
	MessageID   int64                `json:"message_id"`
	Text        string               `json:"text"`
	ParseMode   string               `json:"parse_mode,omitempty"`
	ReplyMarkup InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type DeleteMessageRequest struct {
	ChatID    string `json:"chat_id"`
	MessageID int64  `json:"message_id"`
}

type DeleteMessageResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int64  `json:"error_code"`
	Description string `json:"description"`
}

type Bot struct {
	config     Config
	db         *sql.DB
	httpClient *http.Client
}

func NewBot(config Config) (*Bot, error) {
	db, err := sql.Open("sqlite3", config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &Bot{
		config:     config,
		db:         db,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}, nil
}

func createTables(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS stories (
		id INTEGER PRIMARY KEY,
		message_id INTEGER NOT NULL,
		last_save DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := db.Exec(query)
	return err
}

func (b *Bot) telegramAPI(method string) string {
	return TelegramAPIBase + b.config.BotKey + "/" + method
}

func (b *Bot) newsURL(id int64) string {
	return "https://news.ycombinator.com/item?id=" + strconv.FormatInt(id, 10)
}

func (b *Bot) itemURL(id int64) string {
	return fmt.Sprintf("%s/item/%d.json", HackerNewsAPIBase, id)
}

func (b *Bot) topStoriesURL() string {
	return fmt.Sprintf("%s/topstories.json?orderBy=\"$key\"&limitToFirst=%d", HackerNewsAPIBase, BatchSize)
}

func (b *Bot) getTopStories() ([]int64, error) {
	resp, err := b.httpClient.Get(b.topStoriesURL())
	if err != nil {
		return nil, fmt.Errorf("failed to get top stories: %w", err)
	}
	defer resp.Body.Close()

	var stories []int64
	if err := json.NewDecoder(resp.Body).Decode(&stories); err != nil {
		return nil, fmt.Errorf("failed to decode top stories: %w", err)
	}

	return stories, nil
}

func (b *Bot) getStoryDetails(id int64) (*Story, error) {
	resp, err := b.httpClient.Get(b.itemURL(id))
	if err != nil {
		return nil, fmt.Errorf("failed to get story details: %w", err)
	}
	defer resp.Body.Close()

	var story Story
	if err := json.NewDecoder(resp.Body).Decode(&story); err != nil {
		return nil, fmt.Errorf("failed to decode story: %w", err)
	}

	return &story, nil
}

func (s *Story) shouldIgnore() bool {
	return s.Type != "story" ||
		s.Score < ScoreThreshold ||
		s.Descendants < NumCommentsThreshold ||
		s.URL == ""
}

func (s *Story) getReplyMarkup(b *Bot) InlineKeyboardMarkup {
	var scoreSuffix, commentSuffix string
	if s.Score > 100 {
		scoreSuffix = " " + Hot
	}
	if s.Descendants > 100 {
		commentSuffix = " " + Hot
	}

	return InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{
					Text: fmt.Sprintf("Score: %d+%s", s.Score, scoreSuffix),
					URL:  s.URL,
				},
				{
					Text: fmt.Sprintf("Comments: %d+%s", s.Descendants, commentSuffix),
					URL:  b.newsURL(s.ID),
				},
			},
		},
	}
}

func (b *Bot) saveStory(story *Story) error {
	query := `INSERT OR REPLACE INTO stories (id, message_id, last_save) VALUES (?, ?, ?)`
	_, err := b.db.Exec(query, story.ID, story.MessageID, time.Now())
	return err
}

func (b *Bot) getStoredStory(id int64) (*Story, error) {
	query := `SELECT id, message_id, last_save FROM stories WHERE id = ?`
	row := b.db.QueryRow(query, id)

	var story Story
	err := row.Scan(&story.ID, &story.MessageID, &story.LastSave)
	if err != nil {
		return nil, err
	}

	return &story, nil
}

func (b *Bot) sendMessage(story *Story) error {
	if story.shouldIgnore() {
		return nil
	}

	req := SendMessageRequest{
		ChatID:              b.config.ChatID,
		Text:                fmt.Sprintf("<b>%s</b>  %s", story.Title, story.URL),
		ParseMode:           "HTML",
		ReplyMarkup:         story.getReplyMarkup(b),
		DisableNotification: true,
	}

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal send message request: %w", err)
	}

	resp, err := b.httpClient.Post(b.telegramAPI("sendMessage"), "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	var response SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode send message response: %w", err)
	}

	if !response.OK {
		return fmt.Errorf("telegram API error in send message")
	}

	story.MessageID = response.Result.MessageID
	return b.saveStory(story)
}

func (b *Bot) editMessage(story *Story) error {
	if story.shouldIgnore() {
		return nil
	}

	req := EditMessageTextRequest{
		ChatID:      b.config.ChatID,
		MessageID:   story.MessageID,
		Text:        fmt.Sprintf("<b>%s</b>  %s", story.Title, story.URL),
		ParseMode:   "HTML",
		ReplyMarkup: story.getReplyMarkup(b),
	}

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal edit message request: %w", err)
	}

	resp, err := b.httpClient.Post(b.telegramAPI("editMessageText"), "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to edit message: %w", err)
	}
	defer resp.Body.Close()

	return b.saveStory(story)
}

func (b *Bot) deleteMessage(story *Story) error {
	req := DeleteMessageRequest{
		ChatID:    b.config.ChatID,
		MessageID: story.MessageID,
	}

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal delete message request: %w", err)
	}

	resp, err := b.httpClient.Post(b.telegramAPI("deleteMessage"), "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	defer resp.Body.Close()

	var response DeleteMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode delete message response: %w", err)
	}

	if !response.OK && !b.shouldIgnoreDeleteError(&response) {
		return fmt.Errorf("telegram API error in delete message: %s", response.Description)
	}

	query := `DELETE FROM stories WHERE id = ?`
	_, err = b.db.Exec(query, story.ID)
	return err
}

func (b *Bot) shouldIgnoreDeleteError(resp *DeleteMessageResponse) bool {
	return resp.ErrorCode == 400 &&
		(strings.Contains(resp.Description, "message to delete not found") ||
			strings.Contains(resp.Description, "message can't be deleted"))
}

func (b *Bot) poll() error {
	topStories, err := b.getTopStories()
	if err != nil {
		return fmt.Errorf("failed to get top stories: %w", err)
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for _, storyID := range topStories {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			storedStory, err := b.getStoredStory(id)
			if err == sql.ErrNoRows {
				story, err := b.getStoryDetails(id)
				if err != nil {
					log.Printf("Error getting story details for %d: %v", id, err)
					return
				}

				if err := b.sendMessage(story); err != nil {
					log.Printf("Error sending message for story %d: %v", id, err)
				} else {
					log.Printf("Sent new story: %d - %s", story.ID, story.Title)
				}
			} else if err != nil {
				log.Printf("Error checking stored story %d: %v", id, err)
			} else {
				story, err := b.getStoryDetails(id)
				if err != nil {
					log.Printf("Error getting story details for %d: %v", id, err)
					return
				}

				story.MessageID = storedStory.MessageID
				if err := b.editMessage(story); err != nil {
					log.Printf("Error editing message for story %d: %v", id, err)
				} else {
					log.Printf("Updated story: %d - %s", story.ID, story.Title)
				}
			}
		}(storyID)
	}

	wg.Wait()
	return nil
}

func (b *Bot) cleanup() error {
	oneDayAgo := time.Now().Add(-CleanupInterval)
	query := `SELECT id, message_id FROM stories WHERE last_save <= ?`
	rows, err := b.db.Query(query, oneDayAgo)
	if err != nil {
		return fmt.Errorf("failed to query old stories: %w", err)
	}
	defer rows.Close()

	var stories []Story
	for rows.Next() {
		var story Story
		if err := rows.Scan(&story.ID, &story.MessageID); err != nil {
			log.Printf("Error scanning story: %v", err)
			continue
		}
		stories = append(stories, story)
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5)

	for _, story := range stories {
		wg.Add(1)
		go func(s Story) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := b.deleteMessage(&s); err != nil {
				log.Printf("Error deleting message for story %d: %v", s.ID, err)
			} else {
				log.Printf("Deleted old story: %d", s.ID)
			}
		}(story)
	}

	wg.Wait()
	return nil
}

func (b *Bot) run() {
	pollTicker := time.NewTicker(PollInterval)
	cleanupTicker := time.NewTicker(CleanupInterval)
	defer pollTicker.Stop()
	defer cleanupTicker.Stop()

	log.Printf("Bot started. Polling every %v, cleanup every %v", PollInterval, CleanupInterval)

	if err := b.poll(); err != nil {
		log.Printf("Initial poll error: %v", err)
	}

	for {
		select {
		case <-pollTicker.C:
			if err := b.poll(); err != nil {
				log.Printf("Poll error: %v", err)
			}
		case <-cleanupTicker.C:
			if err := b.cleanup(); err != nil {
				log.Printf("Cleanup error: %v", err)
			}
		}
	}
}

func (b *Bot) Close() error {
	return b.db.Close()
}

func loadConfig() Config {
	botKey := os.Getenv("BOT_KEY")
	if botKey == "" {
		log.Fatal("BOT_KEY environment variable is required")
	}

	chatID := os.Getenv("CHAT_ID")
	if chatID == "" {
		chatID = "@@hacker_news_wooo"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "stories.db"
	}

	return Config{
		BotKey: botKey,
		ChatID: chatID,
		DBPath: dbPath,
	}
}

func main() {
	config := loadConfig()

	bot, err := NewBot(config)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}
	defer bot.Close()

	bot.run()
}

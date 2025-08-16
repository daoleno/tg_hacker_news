# Telegram Hacker News Bot

A Telegram bot that fetches top stories from Hacker News and posts them to a Telegram channel with real-time updates.

## Features

- 🔥 Fetches top 30 stories from Hacker News API
- 📊 Filters high-quality content (score ≥50, comments ≥5)
- 🤖 Posts to Telegram channel with inline buttons
- 🔄 Real-time updates of scores and comment counts
- 🧹 Auto-cleanup of messages older than 24 hours
- 💾 JSON file storage for tracking posted stories
- 🚀 Single Go binary with zero dependencies

## Quick Start

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `BOT_KEY` | Telegram bot token | - | ✅ |
| `CHAT_ID` | Target channel/chat ID | `@hacker_news_wooo` | ❌ |
| `DATA_PATH` | JSON data file path | `stories.json` | ❌ |

### Local Development

1. **Get a Telegram Bot Token**
   - Message [@BotFather](https://t.me/botfather) on Telegram
   - Create a new bot with `/newbot`
   - Copy the bot token

2. **Set up your channel**
   - Create a Telegram channel
   - Add your bot as an administrator
   - Get the channel ID (e.g., `@your_channel`)

3. **Clone and run**
   ```bash
   git clone <repository>
   cd tg_hacker_news
   
   # Set environment variables
   export BOT_KEY="your_bot_token_here"
   export CHAT_ID="@your_channel"
   
   # Run the bot
   go mod tidy
   go run main.go
   ```

### Docker

```bash
# Build and run with Docker
docker build -t tg-hacker-news .
docker run -d \
  -e BOT_KEY="your_bot_token_here" \
  -e CHAT_ID="@your_channel" \
  -v $(pwd)/data:/app/data \
  --name hacker-news-bot \
  tg-hacker-news
```

### Docker Compose

```bash
# Copy and edit environment variables
cp .env.example .env
nano .env

# Start the service
docker-compose up -d

# View logs
docker-compose logs -f
```

## Configuration

### Bot Behavior

The bot operates with these default settings:

- **Poll Interval**: 5 minutes
- **Cleanup Interval**: 24 hours
- **Score Threshold**: 50 points
- **Comments Threshold**: 5 comments
- **Batch Size**: 30 top stories

These can be modified in the source code if needed.

### Message Format

Each story is posted with:
- **Title**: Bold story title with direct link
- **Score Button**: Shows current score with 🔥 if >100
- **Comments Button**: Shows comment count with 🔥 if >100, links to HN discussion

## How It Works

1. **Polling**: Every 5 minutes, fetches top 30 stories from Hacker News API
2. **Filtering**: Only posts stories that meet quality thresholds
3. **Tracking**: Stores story ID and message ID in JSON file
4. **Updates**: If story already posted, updates the message with new scores
5. **Cleanup**: Deletes messages older than 24 hours to keep channel clean

## Data Storage

Stories are stored in a JSON file with the following structure:

```json
{
  "stories": {
    "123456": {
      "id": 123456,
      "message_id": 789,
      "last_save": "2023-12-01T10:00:00Z"
    }
  }
}
```

## API Endpoints Used

- **Hacker News**: `https://hacker-news.firebaseio.com/v0/`
  - `topstories.json` - Get top story IDs
  - `item/{id}.json` - Get story details
- **Telegram**: `https://api.telegram.org/bot{token}/`
  - `sendMessage` - Post new stories
  - `editMessageText` - Update existing stories
  - `deleteMessage` - Remove old stories

## Monitoring

Check bot status:
```bash
# View logs
docker-compose logs -f

# Check data file
cat data/stories.json | jq '.stories | length'

# Monitor bot health
curl -s "https://api.telegram.org/bot$BOT_KEY/getMe"
```

## Troubleshooting

### Common Issues

1. **Bot not posting**: Check bot token and channel permissions
2. **Permission denied**: Ensure bot is admin in target channel
3. **Database locked**: Check file permissions in data directory
4. **Rate limiting**: Bot includes automatic retry logic

### Debug Mode

Add debug logging by modifying the code:
```go
log.SetFlags(log.LstdFlags | log.Lshortfile)
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Acknowledgments

Based on [yegle-bots](https://github.com/yegle/yegle-bots) with improvements:
- Removed Google App Engine dependencies
- Added Docker support
- Simplified deployment
- Enhanced error handling
- Optimized performance
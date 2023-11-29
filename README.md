# Markov Chain Bot for Telegram made in Go

## Instructions

Execute using:

```bash
TOKEN=your_bot_token go run .
```

Persistent data will be stored under `data`

A Dockerfile is provided if you want to use Docker or Podman to execute the bot, you just need to expose `/app/data` via a volume and set the TOKEN variable
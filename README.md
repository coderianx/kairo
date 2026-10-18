<div align="center">
	<img src="assets/logo-tile.png" width="104" alt="Kairo">

</div>

---

Kairo connects Groq, OpenRouter and any **OpenAI-compatible provider** to a single sleek desktop app. Your keys, history and model choices all stay **on your own machine** — no cloud, no telemetry.

## Features

- **Searchable model picker** — your provider's full `/models` catalog in one popup, filtered as you type
- **Conversation history** — chats are stored in local SQLite; every request sends the last **15 messages** to the model as context
- **Encrypted keys** — API keys are sealed with **AES-256-GCM** and never stored in plain text
- **Smart chat titles** — sidebar names are derived automatically from your first message
- **Frameless UI** — a soft dark theme: your words on the right, the model's answer on the left with a brand avatar
- **Fast** — a solid Go binary does the heavy lifting; Electron only draws the window

<div align="center">
	<a href="web/screenshots/welcome.png"><img src="web/screenshots/welcome.png" width="49%" alt="Welcome screen"></a>
	<a href="web/screenshots/settings.png"><img src="web/screenshots/settings.png" width="49%" alt="Settings screen"></a>
</div>

## Getting started

**Requirements:** Go 1.27+ · Node.js 20+ (tested with Electron 44)

```bash
git clone https://github.com/coderianx/kairo.git
cd kairo
npm install
npm run build:backend   # builds the Go backend (kairo-server)
npm start
```

> After any change to the Go code, run `npm run build:backend` again — Electron cannot start without the `kairo-server` binary.

### Linux AppImage

```bash
npm run dist
```

Output: `dist/Kairo-<version>.AppImage` — the Go backend is embedded in the package, ready for single-file distribution.

## Usage

1. **Settings → connect a provider:** pick the Groq or OpenRouter card, paste your API key and hit Apply — the key is encrypted and stored in your local database.
2. Open a conversation with **New Chat**; the sidebar name is derived automatically from your first message.
3. Pick a **provider and model** from the composer pills; models are fetched from your provider, searchable as you type.
4. Type and hit **Enter** — every answer is produced with the last 15 messages of the conversation as context.

## Privacy

**Kairo Cares About Your Privacy** — your keys never leave your machine and your chats never leave your disk. There is no cloud in Kairo, and no telemetry.

## Data

- `~/.kairo/kairo.db` — providers, chats and messages (SQLite, WAL mode)
- `~/.kairo/secret.key` — generated 32-byte secret used for key encryption (permissions `0600`)
- `KAIRO_SECRET_KEY` — optionally provide a fixed hex key instead

## Architecture

`ui (electron) → ipc → kairo-server (go, :8080) → provider api`

Settings, chats and messages live in local SQLite; model requests go through kairo-server to the configured provider with the decrypted key.

## Development

```bash
go test ./...                          # Go tests
npm run build:backend && npm start     # run after changes
npm run dist                           # package an AppImage
```

## Contributing

PRs and issues are welcome — for large changes, please open an issue first so we can discuss the approach.

## License

[MIT](LICENSE) — © 2026 coderianx

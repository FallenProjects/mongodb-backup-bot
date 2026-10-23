# MongoDB Backup Bot

A Telegram bot for MongoDB backups and restores, built with Go and [gotdbot](https://github.com/AshokShau/gotdbot). Includes formatted messages, styled inline buttons and paginated database selection.

## Features

- Back up all databases in a URI's scope or select one database.
- Send compressed `.gz` archives or ZIP files containing BSON and metadata.
- Restore backups by replying to their Telegram documents.
- Wipe user databases after confirmation, excluding `admin`, `config` and `local`.
- Remove temporary backup and download files after processing.

## Docker

```sh
cp .env.example .env
```

Fill in your Telegram credentials, then start the bot:

```sh
docker compose up -d --build
docker compose logs -f bot
```

The image includes TDLib 1.8.69 and MongoDB Database Tools 100.19.1, supports Linux amd64 and arm64, and runs as a non-root user. The `bot-data` volume holds the Telegram session and temporary files under `/data`.

To build and run without Compose:

```sh
docker build -t mongodb-backup-bot .
docker run -d --name mongodb-backup-bot --restart unless-stopped --init \
  --env-file .env -v mongodb-bot-data:/data mongodb-backup-bot
```

MongoDB must be reachable from the container. `localhost` refers to the container itself. For Atlas, allow your server's outbound IP in the project's network access settings.

## Configuration

Get `API_ID` and `API_HASH` from [my.telegram.org](https://my.telegram.org), and `TOKEN` from [@BotFather](https://t.me/BotFather).

| Variable                   | Default        | Description                                            |
|----------------------------|----------------|--------------------------------------------------------|
| `API_ID`                   | Required       | Telegram API ID                                        |
| `API_HASH`                 | Required       | Telegram API hash                                      |
| `TOKEN`                    | Required       | Bot token                                              |
| `OWNER_ID`                 | `0`            | Bot owner's Telegram user ID; does not restrict access |
| `MONGODB_BACKUP_PAGE_SIZE` | `8`            | Databases per page, from 1 to 40                       |

Local execution loads `.env` without overriding existing environment variables. All users can use the bot regardless of `OWNER_ID`. Database commands are restricted to private chats.

## Commands

| Command                 | Action                                                |
|-------------------------|-------------------------------------------------------|
| `/start`, `/help`       | Open help                                             |
| `/mongo <URI>`          | Open the database menu; default format is `.gz`       |
| `/mongo <URI> {gz}`     | Compressed MongoDB archive                            |
| `/mongo <URI> {zip}`    | ZIP containing BSON and JSON metadata                 |
| `/mongo <URI> {json}`   | Alias for `{zip}`, not a plain JSON export            |
| `/mongo <URI> {import}` | Restore the document you reply to, after confirmation |
| `/ping`                 | Latency and uptime                                    |
| `/privacy`              | Data handling information                             |

Flags can go before or after the URI. `{gz}` takes precedence over `{zip}` or `{json}`. A bare MongoDB URI also opens the menu.

### Back up

```text
/mongo mongodb://user:password@host:27017/?authSource=admin
/mongo mongodb+srv://user:password@cluster.example.com/mydb {zip}
```

Choose **Back up all** or **Select database**. A URI containing `/mydb` limits backup, selection and wipe to that database. A URI without a database path operates on the accessible databases on that server.

### Restore

Reply to a backup document with:

```text
/mongo mongodb://user:password@destination:27017/?authSource=admin {import}
```

Press **Restore backup**. Use a URI without a database path; database names come from the backup. Set `authSource` explicitly when needed.

`.gz` files must come from `mongodump --archive --gzip`. ZIP files must have database directories at the archive root, as created by this bot. Plain JSON files are unsupported. Restore keeps existing collections and stops on error; duplicate documents or other failures can leave a partial restore.

### Wipe

Choose **Wipe databases**, then confirm. The bot deletes user databases in the URI's scope. If the server denies dropping a database, it attempts to drop its non-system collections. A failed wipe may be partial.

## Local usage

Install Go 1.27+ and [MongoDB Database Tools](https://www.mongodb.com/try/download/database-tools) with `mongodump` and `mongorestore` on `PATH`.

Set your credentials in `.env`, then run:

```sh
go mod download
go generate
go run .
```

```sh
go build -o bin/mongodb-backup-bot .
go vet ./...
```

## Data handling

The bot reads URIs from your Telegram command when you press a button. Keep the original command available while using its menu. URIs are omitted from backup captions and MongoDB tool logs; tools receive them through a temporary configuration file instead of process arguments.

Backup and download files are removed after processing. TDLib stores its session on disk. A crash can leave temporary files in `backups` or cached downloads in `database/files`; remove abandoned files when the bot is stopped. Commands and backups remain in Telegram until deleted. Use a trusted bot deployment and allow enough disk space for your backups.

## License

[MIT](LICENSE)

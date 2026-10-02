# Reaktor

Reaktor is a discord bot that supports subcommands to configure specific actions.

It was built for k8s, but it can be used with locally and docker.

## Admin-only subcommands:

| Command          | Description                                           |
| ---------------- | ----------------------------------------------------- |
| /config channel  | Set the channel the panel is posted in                |
| /config role     | Set the role granted access to see created channels   |
| /config prefix   | Set the base name for created channels (e.g., ticket) |
| /config category | Set which category ticket channels go in              |
| /config message  | Send the panel text as your next message              |

Ticket channels are named `<prefix>-<n>`, counting up from 1.

Default category is the same as the channel for the panel is in but can be set.

`/config message` requires channel, role, and prefix to already be set.

After sending the panel text, Reaktor replies with a confirmation:
react ✅ to post the panel, or ❌ to redo the message.

## Required permissions

Discord creates a role for Reaktor with these permissions:

- View Channels
- Send Messages
- Read Message History
- Add Reactions
- Manage Channels
- Manage Roles

Only these are needed for Reaktor to work.

## Discord token

Reaktor needs a bot token in the `DISCORD_TOKEN` environment variable.

Locally:

```sh
DISCORD_TOKEN=your-bot-token
go run .
```

Docker, via an env file:

```sh
# .env
DISCORD_TOKEN=your-bot-token
```

```sh
docker run --env-file .env ghcr.io/archrz/reaktor
```

k8s, via a Secret the deployment reads with `secretKeyRef`:

```yaml
apiVersion: v1
kind: Secret
metadata:
    name: reaktor-secrets
stringData:
    discord_token: your-bot-token
```

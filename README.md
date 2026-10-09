# Reaktor

Reaktor is a discord bot that supports subcommands to configure specific actions.

## Admin-only subcommands:

| Command          | Description                                          |
| ---------------- | ---------------------------------------------------- |
| /config channel  | Set the channel the panel is posted in               |
| /config role     | Set the role granted access to see created channels  |
| /config prefix   | Set the base name for created channels (e.g. ticket) |
| /config category | Set which category ticket channels go in             |
| /config message  | Send the panel text as your next message             |

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
docker run --env-file .env -v reaktor-data:/data ghcr.io/archrz/reaktor
```

## Persistence

Reaktor stores per-guild config under `/data/configs`. Mount a volume at
`/data` so it survives restarts (the `-v reaktor-data:/data` above does this
for Docker).

## Kubernetes

Reads the token from a Secret with `secretKeyRef`, persists `/data` with a
PersistentVolumeClaim, and exposes `/healthz` on port 8080 for probes.

Below is a example deployment.yaml

```yaml
apiVersion: v1
kind: Secret
metadata:
    name: reaktor-secrets
stringData:
    discord_token: your-bot-token
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
    name: reaktor-data
spec:
    accessModes: [ReadWriteOnce]
    resources:
        requests:
            storage: 1Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
    name: reaktor
spec:
    replicas: 1
    strategy:
        type: Recreate
    selector:
        matchLabels:
            app: reaktor
    template:
        metadata:
            labels:
                app: reaktor
        spec:
            containers:
                - name: reaktor
                  image: ghcr.io/archrz/reaktor
                  env:
                      - name: DISCORD_TOKEN
                        valueFrom:
                            secretKeyRef:
                                name: reaktor-secrets
                                key: discord_token
                  ports:
                      - containerPort: 8080
                  livenessProbe:
                      httpGet:
                          path: /healthz
                          port: 8080
                      initialDelaySeconds: 10
                      periodSeconds: 30
                  volumeMounts:
                      - name: data
                        mountPath: /data
            volumes:
                - name: data
                  persistentVolumeClaim:
                      claimName: reaktor-data
```

# Reaktor

A ticket style discord bot that is configurable.

Admin-only subcommands, set independently and changeable any time:

| Command         | Description                                           |
| --------------- | ----------------------------------------------------- |
| /config channel | Set the channel the panel is posted in                |
| /config role    | Set the role granted access to see created channels   |
| /config prefix  | Set the base name for created channels (e.g., ticket) |
| /config message | Send the panel text as your next message              |

`/config message` requires channel, role, and prefix to already be set. After
sending the panel text, the bot replies with a confirmation: react ✅ to post
the panel, or ❌ to redo the message.

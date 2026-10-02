# Reaktor

A ticket style discord bot that is configurable. Back from the dead.

Admin-only subcommands, changeable any time:

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

After sending the panel text, the bot replies with a confirmation:
react ✅ to post the panel, or ❌ to redo the message.

# API Overview

WSAPI exposes 74 REST API endpoints grouped by domain. Full request/response schemas are documented in the [OpenAPI spec](https://github.com/wsapi-chat/wsapi-app/blob/main/openapi/wsapi-api.yml). [![Swagger UI](https://img.shields.io/badge/Swagger_UI-85EA2D?logo=swagger&logoColor=black)](https://petstore.swagger.io/?url=https://raw.githubusercontent.com/wsapi-chat/wsapi-app/main/openapi/wsapi-api.yml)

## Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |

## Instances (Admin, Multi Mode Only)

Requires `X-Api-Key` header with the admin API key.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/admin/instances` | Create a new instance |
| GET | `/admin/instances` | List all instances |
| GET | `/admin/instances/{id}` | Get instance details |
| DELETE | `/admin/instances/{id}` | Delete an instance |
| PUT | `/admin/instances/{id}/config` | Update instance configuration |
| PUT | `/admin/instances/{id}/restart` | Restart an instance |

## Session

| Method | Path | Description |
|--------|------|-------------|
| GET | `/session/qr` | Get QR code for pairing (PNG image) |
| GET | `/session/qr/text` | Get QR code as text |
| GET | `/session/pair-code/{phone}` | Get pair code for phone number |
| GET | `/session/status` | Get session status |
| POST | `/session/logout` | Logout from WhatsApp |
| POST | `/session/flush-history` | Flush cached history sync messages |

## Messages

| Method | Path | Description |
|--------|------|-------------|
| POST | `/messages/text` | Send a text message |
| POST | `/messages/image` | Send an image message |
| POST | `/messages/video` | Send a video message |
| POST | `/messages/audio` | Send an audio message |
| POST | `/messages/voice` | Send a voice message |
| POST | `/messages/document` | Send a document message |
| POST | `/messages/sticker` | Send a sticker message |
| POST | `/messages/contact` | Send a contact card |
| POST | `/messages/location` | Send a location message |
| POST | `/messages/link` | Send a link preview message |
| POST | `/messages/{messageId}/reaction` | React to a message |
| POST | `/messages/{messageId}/edit` | Edit a sent message |
| POST | `/messages/{messageId}/read` | Mark a message as read |
| POST | `/messages/{messageId}/star` | Star or unstar a message |
| POST | `/messages/{messageId}/pin` | Pin or unpin a message |
| POST | `/messages/{messageId}/delete` | Delete a message for everyone |
| POST | `/messages/{messageId}/delete-for-me` | Delete a message for me only |

## Groups

| Method | Path | Description |
|--------|------|-------------|
| GET | `/groups` | List joined groups |
| POST | `/groups` | Create a new group |
| GET | `/groups/{id}` | Get group info |
| PUT | `/groups/{id}/name` | Set group name |
| PUT | `/groups/{id}/description` | Set group description |
| POST | `/groups/{id}/picture` | Set group picture |
| POST | `/groups/{id}/leave` | Leave a group |
| GET | `/groups/{id}/participants` | Get group participants |
| PUT | `/groups/{id}/participants` | Update group participants |
| GET | `/groups/{id}/invite-link` | Get group invite link |
| POST | `/groups/{id}/invite-link/reset` | Reset group invite link |
| PUT | `/groups/{id}/settings/announce` | Set group announce mode |
| PUT | `/groups/{id}/settings/locked` | Set group locked mode |
| PUT | `/groups/{id}/settings/join-approval` | Set group join approval mode |
| PUT | `/groups/{id}/settings/member-add-mode` | Set member add mode |
| POST | `/groups/join/link` | Join a group via invite link |
| POST | `/groups/join/invite` | Join a group via invite message |
| GET | `/groups/invite/{code}` | Get group info from invite code |
| GET | `/groups/{id}/requests` | Get pending join requests |
| PUT | `/groups/{id}/requests` | Approve or reject join requests |

## Communities

| Method | Path | Description |
|--------|------|-------------|
| GET | `/communities` | List joined communities |
| POST | `/communities` | Create a new community |
| GET | `/communities/{id}` | Get community info |
| POST | `/communities/{id}/leave` | Leave a community |
| PUT | `/communities/{id}/name` | Set community name |
| PUT | `/communities/{id}/description` | Set community description |
| POST | `/communities/{id}/picture` | Set community picture |
| PUT | `/communities/{id}/settings/locked` | Set community locked mode |
| GET | `/communities/{id}/participants` | Get community participants |
| PUT | `/communities/{id}/participants` | Update community participants |
| GET | `/communities/{id}/invite-link` | Get community invite link |
| POST | `/communities/{id}/invite-link/reset` | Reset community invite link |
| GET | `/communities/{id}/groups` | Get community sub-groups |
| POST | `/communities/{id}/groups` | Create a group within a community |
| POST | `/communities/{id}/groups/link` | Link an existing group to a community |
| DELETE | `/communities/{id}/groups/{groupId}` | Unlink a group from a community |

## Contacts

| Method | Path | Description |
|--------|------|-------------|
| GET | `/contacts` | List all contacts |
| POST | `/contacts` | Create or update a contact |
| POST | `/contacts/sync` | Sync contacts from WhatsApp |
| GET | `/contacts/{id}` | Get a specific contact |
| GET | `/contacts/blocklist` | Get blocked contacts |
| PUT | `/contacts/{id}/block` | Block a contact |
| PUT | `/contacts/{id}/unblock` | Unblock a contact |

## Users

| Method | Path | Description |
|--------|------|-------------|
| GET | `/users/me/profile` | Get own profile info |
| PUT | `/users/me/profile` | Update own profile |
| PUT | `/users/me/presence` | Set presence state |
| GET | `/users/me/privacy` | Get privacy settings |
| PUT | `/users/me/privacy` | Update a privacy setting |
| POST | `/users/check` | Bulk check phone numbers on WhatsApp |
| GET | `/users/{phone}/check` | Check if a phone number is on WhatsApp |
| GET | `/users/{phone}/profile` | Get user profile info |

## Media

| Method | Path | Description |
|--------|------|-------------|
| GET | `/media/download` | Download media by ID |

## Chats

| Method | Path | Description |
|--------|------|-------------|
| GET | `/chats` | List all known chats |
| GET | `/chats/{chatId}` | Get chat info |
| DELETE | `/chats/{chatId}` | Delete a chat |
| GET | `/chats/{chatId}/picture` | Get chat profile picture |
| GET | `/chats/{chatId}/business` | Get business profile for a chat |
| PUT | `/chats/{chatId}/presence` | Send chat presence (typing, paused, recording) |
| PUT | `/chats/{chatId}/presence/subscribe` | Subscribe to presence updates |
| PUT | `/chats/{chatId}/ephemeral` | Set disappearing messages timer |
| PUT | `/chats/{chatId}/mute` | Mute or unmute a chat |
| PUT | `/chats/{chatId}/pin` | Pin or unpin a chat |
| PUT | `/chats/{chatId}/archive` | Archive or unarchive a chat |
| PUT | `/chats/{chatId}/read` | Mark a chat as read or unread |
| POST | `/chats/{chatId}/messages` | Request on-demand message history |
| POST | `/chats/{chatId}/clear` | Clear all messages from a chat |

## Calls

| Method | Path | Description |
|--------|------|-------------|
| POST | `/calls/{callId}/reject` | Reject an incoming call |

## Newsletters

| Method | Path | Description |
|--------|------|-------------|
| GET | `/newsletters` | List subscribed newsletters |
| POST | `/newsletters` | Create a newsletter |
| GET | `/newsletters/invite/{code}` | Get newsletter info by invite code |
| GET | `/newsletters/{id}` | Get newsletter info |
| PUT | `/newsletters/{id}/subscription` | Subscribe or unsubscribe |
| PUT | `/newsletters/{id}/mute` | Mute or unmute a newsletter |

## Status

| Method | Path | Description |
|--------|------|-------------|
| GET | `/status/privacy` | Get status privacy settings |
| POST | `/status/text` | Post a text status |
| POST | `/status/image` | Post an image status |
| POST | `/status/video` | Post a video status |
| POST | `/status/{messageId}/delete` | Delete a status update |

# Real-time 1:1 chat feature

## Context

The app currently has no way for friends to message each other — only structured interactions (challenges, rewards, transactions) exist. The goal is a WebSocket-backed chat system so friends can exchange free-text messages in real time, with history persisted so it survives reconnects and works whether the client is the native Android app or the Flutter-Web build running in Safari on iPhone (no native iOS app exists yet).

Decisions locked in during planning:
- **Scope: 1:1 only**, gated by the existing friendship relationship — no group chat / conversation-room entity (the old `group.go` model is already gone from this codebase).
- **WebSocket auth: JWT via query param** (`wss://…/api/chat/ws?token=<jwt>`) — required because Flutter Web in Safari can't set custom headers during a WS handshake, and this needs one code path that works identically on native Android and browser iPhone.
- **Storage: PostgreSQL only** (already the sole datastore), single backend instance — no Redis/Cassandra, no horizontal-scaling concerns at this project's scale.
- **Multi-device**: a user may have several live connections at once (e.g. phone + web tab) — the hub must support fan-out to all of them, and a sent message is echoed back to the sender's *other* connected devices too, so multiple open sessions stay in sync.
- **History access**: friendship is only checked when **sending** a new message. Reading history / marking as read is **not** gated on current friendship status, so unfriending doesn't retroactively lock either party out of past messages.
- **Push notifications are explicitly out of scope** for this pass (to be layered in later via FCM).

This follows the codebase's established Repository → Service → Handler domain pattern (see `internal/domain/friendship/` and `internal/domain/challenge/` as the closest analogs) so it's consistent with `user`, `friendship`, `challenge`, `reward`, `transaction`.

## Files to add

### `internal/models/message.go`
```go
type Message struct {
    ID          uuid.UUID  `gorm:"primaryKey;type:uuid" json:"id"`
    SenderID    uuid.UUID  `gorm:"type:uuid;not null;index:idx_message_sender_recipient,priority:1;index:idx_message_recipient_sender,priority:2" json:"senderId"`
    RecipientID uuid.UUID  `gorm:"type:uuid;not null;index:idx_message_sender_recipient,priority:2;index:idx_message_recipient_sender,priority:1" json:"recipientId"`
    Body        string     `gorm:"not null" json:"body"`
    CreatedAt   time.Time  `gorm:"index:idx_message_sender_recipient,priority:3;index:idx_message_recipient_sender,priority:3" json:"createdAt"`
    ReadAt      *time.Time `json:"readAt"`
}
```
Two named composite indexes cover both query directions of `(sender, recipient, time)`. No `uniqueIndex` (unlike `Friendship`'s `idx_friendship_pair`) — many messages can exist between the same pair. No `UpdatedAt` — messages are immutable except the `ReadAt` mark, updated via a direct `Updates(map[string]interface{}{...})` call, same idiom as `Friendship.RespondedAt`. Cap `Body` at 4000 chars, enforced in the service (`400` if exceeded), as a sanity bound — no existing precedent for a max length elsewhere, this is a reasonable new convention.

### `internal/domain/chat/repository.go`
Standard `Repository` interface + unexported `repository{db *gorm.DB}` + `NewRepository(db *gorm.DB) Repository`, matching `friendship/repository.go`'s shape exactly:
- `Create(m *models.Message) error` — plain `r.db.Create(m).Error`.
- `FindConversation(user1ID, user2ID uuid.UUID, before *time.Time, limit int) ([]models.Message, error)` — reuses the exact `(sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?)` idiom already present in `friendship/repository.go`'s `FindBetween`, plus `created_at < ?` when `before != nil`, `Order("created_at DESC").Limit(limit)`. Returns newest-first; reversing to chronological order is a **service**-layer concern, not the repo's.
- `MarkAsRead(senderID, recipientID uuid.UUID) error` — `Where("sender_id = ? AND recipient_id = ? AND read_at IS NULL", ...).Updates(map[string]interface{}{"read_at": gorm.Expr("NOW()")})`, mirroring `Friendship.UpdateStatus`'s `NOW()` idiom. Marks messages sent *by* `senderID` *to* `recipientID` as read — the caller (service) must pass `(otherPersonID, callerID)`, since it's the recipient who's doing the marking.
- `CountUnread(recipientID, senderID uuid.UUID) (int64, error)` — optional/cheap, not wired to any route yet.
- No `Transact`/`*Tx` variants — no multi-row atomic writes needed here.

```go
func (r *repository) Create(m *models.Message) error {
	return r.db.Create(m).Error
}

func (r *repository) FindConversation(user1ID, user2ID uuid.UUID, before *time.Time, limit int) ([]models.Message, error) {
	query := r.db.Where(
		"(sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?)",
		user1ID, user2ID, user2ID, user1ID,
	)
	if before != nil {
		query = query.Where("created_at < ?", *before)
	}

	var messages []models.Message
	err := query.Order("created_at DESC").Limit(limit).Find(&messages).Error
	return messages, err
}

func (r *repository) MarkAsRead(senderID, recipientID uuid.UUID) error {
	return r.db.Model(&models.Message{}).
		Where("sender_id = ? AND recipient_id = ? AND read_at IS NULL", senderID, recipientID).
		Updates(map[string]interface{}{"read_at": gorm.Expr("NOW()")}).Error
}

func (r *repository) CountUnread(recipientID, senderID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Message{}).
		Where("recipient_id = ? AND sender_id = ? AND read_at IS NULL", recipientID, senderID).
		Count(&count).Error
	return count, err
}
```

Notes:
- `FindConversation` reuses the two-directional `OR` pattern from `friendship.FindBetween`, since either user could be the sender. It returns newest-first (`ORDER BY created_at DESC`) — reversing to chronological order for display is left to the service layer.
- `MarkAsRead` mirrors `friendship.UpdateStatus`'s use of `gorm.Expr("NOW()")` so the timestamp is computed server-side by Postgres, avoiding app/DB clock skew. The `read_at IS NULL` filter makes it safe to call repeatedly.
- Argument order matters: `MarkAsRead(senderID, recipientID)` marks messages sent *by* `senderID` *to* `recipientID` as read. The caller (service) must pass `(otherPersonID, callerID)`, since it's the recipient doing the marking, not the sender.

**Status: implemented** (see `internal/domain/chat/repository.go`).

### `internal/domain/chat/service.go`
`ServiceError`/`ResolveServiceError` boilerplate copied verbatim (identical in every domain today). Two locally-declared narrow interfaces for cross-domain/cross-layer dependencies, same structural-typing trick `challenge/service.go` and `reward/service.go` already use for `FriendService`:
```go
type FriendService interface {
    IsFriend(userA, userB uuid.UUID) (bool, error)
}
type Pusher interface {
    PushToUser(userID uuid.UUID, message *models.Message)
}
```
`friendshipService` (already constructed in `main.go`) satisfies `FriendService` structurally — no changes needed there. The `Hub` (below) satisfies `Pusher` structurally, keeping `gorilla/websocket` **out of `service.go`'s imports entirely** (this codebase's `service.go` files currently have zero gin/http/ws imports — preserve that).

`Service` interface: `SendMessage(ctx, senderID, recipientID uuid.UUID, body string) (*models.Message, error)`, `GetConversation(ctx, callerID, otherID uuid.UUID, before *time.Time, limit int) ([]models.Message, error)`, `MarkAsRead(ctx, callerID, otherID uuid.UUID) error`.

- **`SendMessage`**: reject `senderID == recipientID` (400); check `friendService.IsFriend(senderID, recipientID)` (403 if not friends — the only place friendship is checked); trim/validate `body` non-empty and ≤4000 chars (400); `repo.Create` (500 on error); push to recipient **and** to sender's own other connections via `pusher.PushToUser` (fire-and-forget, no error surfaced — "recipient/sender-device offline" is normal, not a failure, the message is already durably persisted).
- **`GetConversation`**: no friendship check (history stays readable post-unfriend); clamp `limit` server-side (empty/`<=0` → default 50, cap at 100 — new pagination convention, no prior art in this codebase to match); call repo, reverse DESC→ascending before returning.
- **`MarkAsRead`**: no friendship check either, for the same reason; delegates straight to `repo.MarkAsRead(otherID, callerID)` (note the argument order — the caller is the recipient).

### `internal/domain/chat/hub.go` (NOT part of the repo/service/handler triad)
The only file importing `github.com/gorilla/websocket` besides the upgrade call in `handler.go`. Connection registry: `map[uuid.UUID][]*connection` guarded by `sync.RWMutex`, where `connection{conn *websocket.Conn, send chan []byte}`.

Concurrency design (gorilla/websocket allows at most one concurrent reader and one concurrent writer per connection, so this must be enforced structurally, not just by convention):
- `Register`/`Unregister(userID, *connection)` — lock, append/remove from the slice, `Unregister` closes `send` (which lets `writePump` exit and close the socket) and deletes the map key once empty.
- `PushToUser(userID uuid.UUID, message *models.Message)` — marshal once, for each live connection do a **non-blocking** `select { case c.send <- payload: default: }` — a slow/stuck client never blocks delivery to others; the message is still safely in Postgres regardless, so a dropped live push just means that client catches up via history fetch on reconnect.
- Each connection gets exactly two goroutines: `writePump` (the only goroutine calling `conn.WriteMessage`, loops on the `send` channel, includes a ping ticker + `SetWriteDeadline` to detect dead peers) and `readPump` (the only goroutine calling `conn.ReadMessage`, runs inline on the handler's own goroutine rather than spawned separately, since the handler's lifetime should match the connection's).

### `internal/domain/chat/handler.go`
`Handler{service Service, hub *Hub, upgrader websocket.Upgrader}`, `NewHandler(service Service, hub *Hub) *Handler`. `CheckOrigin: func(r *http.Request) bool { return true }` — the existing CORS config in `main.go` only allows `localhost`/`127.0.0.1` origins for REST, which would break WS testing off a LAN IP or real device; since JWT auth already gates the socket, origin-checking is skipped here rather than duplicating/extending that CORS logic.

Routes (`RegisterRoutes(rg *gin.RouterGroup)`, static-before-wildcard comment convention preserved):
- `GET /:friendId/messages` — parses `friendId` (400 on bad UUID), `before` (RFC3339, 400 on bad format), `limit` query params; calls `service.GetConversation`; `ResolveServiceError` → `c.JSON` on failure, else `c.JSON(200, messages)`.
- `POST /:friendId/read` — calls `service.MarkAsRead`; `c.JSON(200, gin.H{"message": "Messages marked as read"})`, matching the `gin.H{"message": ...}` idiom used by `friendship`'s `Decline`/`Unfriend`.
- `ServeWS(c *gin.Context)` (registered separately, see wiring below) — `httputil.ParseUserID(c)` for the caller's ID (works because `RequireAuthWS` populates the same `"userID"` context key `RequireAuth` does, so `ParseUserID` needs zero changes); `upgrader.Upgrade(c.Writer, c.Request, nil)`; on success, wrap the conn, `hub.Register`, spawn `writePump` as a goroutine, run the read loop inline. Each inbound frame is `{"recipientId": "...", "body": "..."}`, unmarshalled and passed to `service.SendMessage`. **This is the only path for sending a message** — there is no REST "send" endpoint by design, since sending only makes sense over a live connection. A service error is sent back down the sender's own socket as a small `{"error": "..."}` frame (not just logged), so a broken client can see what failed. On any read error (disconnect), `hub.Unregister`.

Swaggo doc comments (`@Summary`, `@Tags`, `@Security BearerAuth`, `@Router`) precede the two REST handlers, matching every other domain.

### `internal/middlewares/auth_middleware.go` (edit)
Factor the existing HMAC-parse-and-claims-extract block out of `RequireAuth` into a shared `parseUserIDFromToken(tokenString string) (string, error)` helper, so the logic isn't duplicated. `RequireAuth`'s external behavior for the header path is unchanged. Add a new, additive `RequireAuthWS(c *gin.Context)` that reads `c.Query("token")` instead of the `Authorization` header, calls the same helper, and `c.Set("userID", ...)` on success. Applied **only** to the WS route — REST chat endpoints keep using header-based `RequireAuth` like every other protected route, since Flutter's `http`/`fetch` client for those calls has no header restriction.

## Wiring changes

**`cmd/api/main.go`** — after the existing `friendshipRepo/friendshipService/friendshipHandler` block (chat depends on `friendshipService`):
```go
chatHub := chat.NewHub()
chatRepo := chat.NewRepository(database.DB)
chatService := chat.NewService(chatRepo, friendshipService, chatHub)
chatHandler := chat.NewHandler(chatService, chatHub)
```
`chatHub` is passed where `Pusher` is expected — it satisfies that locally-declared interface structurally, same trick as passing `friendshipService`/`transactionService` into `challenge.NewService` today.

Route registration — REST routes go inside the existing `protected` block like every other domain:
```go
chatHandler.RegisterRoutes(protected.Group("/chat"))
```
The WS route needs a **separate** group using `RequireAuthWS` instead of `RequireAuth`, registered against `api` (not `protected`) — register this block *before* the `protected.Group("/chat")` line, so the static `ws` segment is established first:
```go
wsGroup := api.Group("/chat")
wsGroup.Use(middlewares.RequireAuthWS)
wsGroup.GET("/ws", chatHandler.ServeWS)
```
Resulting endpoints: `GET /api/chat/:friendId/messages`, `POST /api/chat/:friendId/read`, `GET /api/chat/ws?token=<jwt>`.

**`internal/database/database.go`** — append `&models.Message{}` to the existing `AutoMigrate(...)` call.

**`internal/database/seeder.go`** — add `messages` to the `TRUNCATE TABLE ...` list; add a `SeedMessage(sender, recipient models.User, body string) models.Message` helper mirroring `SeedFriendship`'s shape, and seed 2–3 messages between the existing `basten`/`alleece` seed users (one marked read) so `GET /chat/:friendId/messages` is verifiable without needing a WS client first.

**`go.mod`** — `go get github.com/gorilla/websocket@latest && go mod tidy` (no existing WS library in this repo). Note the current `go.mod` marks almost everything `// indirect` including directly-imported packages like `gin-gonic/gin` — `go mod tidy` may reclassify multiple existing lines when run; that's expected cleanup, not a regression to fix around.

## Verification

No Flutter client exists yet, so verify with a small throwaway Go script using `gorilla/websocket`'s client dialer (reuses the dependency already being added):
1. `go run cmd/api/main.go -seed` to reset+seed (creates `basten`/`alleece`, already friends).
2. Start the server; log in both seeded users via the existing `POST /api/auth/login` to get two JWTs.
3. Dial `ws://localhost:8080/api/chat/ws?token=<jwt>` for both users as two separate connections.
4. Send `{"recipientId": "<alleeceUUID>", "body": "hello"}` from basten's socket; assert alleece's socket receives it within ~2s, and assert basten's *own* socket also receives an echo (per the multi-device-sync decision) if a second basten connection is opened.
5. `GET /api/chat/<alleeceUUID>/messages` with basten's JWT → assert the message appears.
6. `POST /api/chat/<bastenUUID>/read` with alleece's JWT → 200; re-fetch history, confirm `readAt` is now set.
7. Close and reconnect alleece's socket with the same token; send another message; confirm delivery still works (proves register/unregister doesn't leak or break re-registration).
8. Send several messages, then verify `?limit=1` + `before=<cursor>` pagination returns older messages without duplicates.
9. Register a third user who is **not** friends with basten; attempt to message them → assert an `{"error": ...}` frame comes back on basten's socket, and `GET /api/chat/<strangerId>/messages` returns 403.
10. Unfriend basten/alleece via the existing friendship endpoint, then confirm `GET /api/chat/<alleeceUUID>/messages` still succeeds for basten (200, history intact) while a new `SendMessage` attempt now returns 403.

## Open decisions (already resolved during planning, kept here for reference)

1. **Route paths** — settled as `/api/chat/:friendId/messages`, `/api/chat/:friendId/read`, `/api/chat/ws`.
2. **CORS/`CheckOrigin` for the WS upgrade** — settled as allow-all (`return true`), relying on JWT auth rather than duplicating the CORS origin logic.
3. **Multi-device echo** — settled as yes: a sent message is also pushed to the sender's own other live connections.
4. **History access after unfriending** — settled as: history stays readable; only sending new messages is gated on current friendship status.
5. **Message body constraints** — settled as a 4000-character cap, enforced in the service.
6. **Unread count endpoint** — left out of v1 scope; `CountUnread` exists on the repository but has no route wired to it yet.

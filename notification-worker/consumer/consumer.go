package consumer

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"notification-worker/email"
	"notification-worker/models"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// Run starts the Kafka consumer loop. Each event triggers two side effects:
//   1. Email notification (create/update only, when UserEmail present)
//   2. Cache invalidation: delete the user's first-page todos cache key
// Both run regardless — they're independent concerns sharing the same event stream.
func Run(ctx context.Context, rdb *redis.Client) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{os.Getenv("KAFKA_BROKER")},
		Topic:       envOrDefault("KAFKA_TOPIC", "todos"),
		GroupID:     envOrDefault("KAFKA_GROUP_ID", "notification-worker"),
		StartOffset: kafka.LastOffset,
	})
	defer reader.Close()

	log.Println("notification-worker consuming Kafka...")

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("kafka read error: %v", err)
			continue
		}

		var event models.TodoEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("kafka decode error: %v (msg=%s)", err, string(msg.Value))
			continue
		}

		// 1. Email — fire on create/update with a recipient.
		if (event.Action == models.ActionCreate || event.Action == models.ActionUpdate) && event.UserEmail != "" {
			subject, text, html := email.RenderTodo(event.Action, event.Todo)
			email.Send(event.UserEmail, "", subject, text, html)
		}

		// 2. Cache invalidation — delete the user's todo-list cache regardless of action.
		if rdb != nil && event.Todo.UserID != "" {
			invalidateUserTodoListCache(ctx, rdb, event.Todo.UserID)
		}
	}
}

// invalidateUserTodoListCache deletes all cached first-page entries for a user.
// Pattern: todos:list:{userID}:* (covers different status filters).
func invalidateUserTodoListCache(ctx context.Context, rdb *redis.Client, userID string) {
	pattern := "todos:list:" + userID + ":*"
	iter := rdb.Scan(ctx, 0, pattern, 100).Iterator()
	keys := []string{}
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		log.Printf("cache invalidate scan error for %s: %v", pattern, err)
		return
	}
	if len(keys) == 0 {
		return
	}
	if err := rdb.Del(ctx, keys...).Err(); err != nil {
		log.Printf("cache invalidate del error for %v: %v", keys, err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

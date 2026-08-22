package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StreamName    = "okf:jobs"
	ConsumerGroup = "okf:workers"
)

// JobMessage es el mensaje consumido del Redis Stream.
type JobMessage struct {
	JobID        string `json:"job_id"`
	UserID       string `json:"user_id"`
	OriginalPath string `json:"original_path"`
	Format       string `json:"format"`
	Filename     string `json:"filename"`
	MsgID        string `json:"-"` // ID interno del mensaje Redis (no se serializa)
}

// RedisConsumer gestiona el consumo de mensajes del Redis Stream.
type RedisConsumer struct {
	client       *redis.Client
	consumerName string
}

// NewRedisConsumer crea una nueva conexión al Redis Stream como consumidor.
func NewRedisConsumer(redisURL, consumerName string) (*RedisConsumer, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("URL de Redis inválida: %w", err)
	}

	client := redis.NewClient(opts)

	// Reintentar conexión
	for i := 0; i < 10; i++ {
		_, err = client.Ping(context.Background()).Result()
		if err == nil {
			break
		}
		log.Printf("[Redis Consumer] Intento %d/10: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a Redis: %w", err)
	}

	// Crear consumer group si no existe
	ctx := context.Background()
	err = client.XGroupCreateMkStream(ctx, StreamName, ConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, fmt.Errorf("error creando consumer group: %w", err)
	}

	log.Printf("[Redis Consumer] Conectado como '%s'", consumerName)
	return &RedisConsumer{client: client, consumerName: consumerName}, nil
}

// ReadMessages lee mensajes del stream en un bucle bloqueante.
// Primero procesa mensajes pendientes (reclamados pero no confirmados), luego mensajes nuevos.
// El channel retornado se cierra cuando se cancela el contexto.
func (c *RedisConsumer) ReadMessages(ctx context.Context) <-chan JobMessage {
	ch := make(chan JobMessage, 10)

	go func() {
		defer close(ch)

		// Primero reclamar mensajes pendientes (de reentregas)
		c.claimPending(ctx, ch)

		// Luego consumir mensajes nuevos
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    ConsumerGroup,
				Consumer: c.consumerName,
				Streams:  []string{StreamName, ">"},
				Count:    1,
				Block:    5 * time.Second,
			}).Result()

			if err != nil {
				if err == redis.Nil || err == context.DeadlineExceeded {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				log.Printf("[Redis Consumer] Error leyendo stream: %v", err)
				time.Sleep(time.Second)
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					jm, err := parseMessage(msg)
					if err != nil {
						log.Printf("[Redis Consumer] Error parseando mensaje %s: %v", msg.ID, err)
						c.acknowledge(ctx, msg.ID)
						continue
					}
					jm.MsgID = msg.ID
					ch <- *jm
				}
			}
		}
	}()

	return ch
}

// claimPending reclama mensajes que estaban pendientes (no confirmados).
// Esto ocurre cuando el worker se reinicia tras un fallo.
func (c *RedisConsumer) claimPending(ctx context.Context, ch chan<- JobMessage) {
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: StreamName,
		Group:  ConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()
	if err != nil {
		return
	}

	for _, p := range pending {
		msgs, err := c.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   StreamName,
			Group:    ConsumerGroup,
			Consumer: c.consumerName,
			MinIdle:  30 * time.Second,
			Messages: []string{p.ID},
		}).Result()
		if err != nil {
			continue
		}
		for _, msg := range msgs {
			jm, err := parseMessage(msg)
			if err != nil {
				c.acknowledge(ctx, msg.ID)
				continue
			}
			jm.MsgID = msg.ID
			ch <- *jm
		}
	}
}

// Acknowledge confirma que un mensaje fue procesado exitosamente.
func (c *RedisConsumer) Acknowledge(ctx context.Context, msgID string) error {
	return c.client.XAck(ctx, StreamName, ConsumerGroup, msgID).Err()
}

// GetDeliveryCount obtiene cuántas veces se intentó procesar el mensaje.
func (c *RedisConsumer) GetDeliveryCount(ctx context.Context, msgID string) int {
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: StreamName,
		Group:  ConsumerGroup,
		Start:  msgID,
		End:    msgID,
		Count:  1,
	}).Result()
	if err != nil || len(pending) == 0 {
		return 1
	}
	return int(pending[0].RetryCount)
}

func (c *RedisConsumer) acknowledge(ctx context.Context, msgID string) {
	_ = c.Acknowledge(ctx, msgID)
}

// Close cierra la conexión.
func (c *RedisConsumer) Close() error {
	return c.client.Close()
}

// parseMessage convierte un mensaje Redis en JobMessage.
func parseMessage(msg redis.XMessage) (*JobMessage, error) {
	payload, ok := msg.Values["payload"].(string)
	if !ok {
		return nil, fmt.Errorf("payload no encontrado en mensaje")
	}
	var jm JobMessage
	if err := json.Unmarshal([]byte(payload), &jm); err != nil {
		return nil, fmt.Errorf("error deserializando payload: %w", err)
	}
	return &jm, nil
}

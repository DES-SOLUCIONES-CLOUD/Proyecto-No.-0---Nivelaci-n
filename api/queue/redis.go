package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"okf-api/models"
)

const (
	// StreamName es el nombre del Redis Stream para trabajos de conversión.
	StreamName = "okf:jobs"
	// ConsumerGroup es el nombre del grupo de consumidores para los workers.
	ConsumerGroup = "okf:workers"
)

// RedisQueue gestiona la publicación de mensajes en Redis Streams.
type RedisQueue struct {
	client *redis.Client
}

// NewRedisQueue crea una nueva conexión a Redis con reintentos.
func NewRedisQueue(redisURL string) (*RedisQueue, error) {
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
		log.Printf("[Redis] Intento %d/10 fallido: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a Redis: %w", err)
	}

	// Crear consumer group si no existe
	ctx := context.Background()
	err = client.XGroupCreateMkStream(ctx, StreamName, ConsumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, fmt.Errorf("error creando consumer group: %w", err)
	}

	log.Println("[Redis] Conexión establecida y stream configurado")
	return &RedisQueue{client: client}, nil
}

// PublishJob publica un mensaje de trabajo en el Redis Stream.
// La API llama a este método y retorna inmediatamente, sin esperar al worker.
func (q *RedisQueue) PublishJob(ctx context.Context, msg *models.JobMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error serializando mensaje: %w", err)
	}

	err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamName,
		Values: map[string]interface{}{
			"payload": string(data),
		},
	}).Err()
	if err != nil {
		return fmt.Errorf("error publicando en Redis Stream: %w", err)
	}

	log.Printf("[Redis] Job %s publicado en stream", msg.JobID)
	return nil
}

// Close cierra la conexión a Redis.
func (q *RedisQueue) Close() error {
	return q.client.Close()
}

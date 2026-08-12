package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIO envuelve el cliente de MinIO y los nombres de los buckets.
type MinIO struct {
	client      *minio.Client
	BucketOrig  string
	BucketBundl string
}

// NewMinIO crea y configura el cliente MinIO, creando los buckets si no existen.
func NewMinIO(endpoint, accessKey, secretKey, bucketOrig, bucketBundl string, useSSL bool) (*MinIO, error) {
	var client *minio.Client
	var err error

	// Reintentar conexión
	for i := 0; i < 10; i++ {
		client, err = minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: useSSL,
		})
		if err == nil {
			// Verificar conectividad
			_, err = client.ListBuckets(context.Background())
			if err == nil {
				break
			}
		}
		log.Printf("[MinIO] Intento %d/10 fallido: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a MinIO: %w", err)
	}

	m := &MinIO{client: client, BucketOrig: bucketOrig, BucketBundl: bucketBundl}

	// Crear buckets si no existen
	for _, bucket := range []string{bucketOrig, bucketBundl} {
		exists, err := client.BucketExists(context.Background(), bucket)
		if err != nil {
			return nil, fmt.Errorf("error verificando bucket %s: %w", bucket, err)
		}
		if !exists {
			if err := client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{}); err != nil {
				return nil, fmt.Errorf("error creando bucket %s: %w", bucket, err)
			}
			log.Printf("[MinIO] Bucket '%s' creado", bucket)
		}
	}

	log.Println("[MinIO] Conexión establecida y buckets verificados")
	return m, nil
}

// UploadObject sube un objeto al bucket especificado.
func (m *MinIO) UploadObject(ctx context.Context, bucket, objectPath string, reader io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, bucket, objectPath, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// DownloadObject descarga un objeto del bucket especificado.
func (m *MinIO) DownloadObject(ctx context.Context, bucket, objectPath string) (*minio.Object, error) {
	return m.client.GetObject(ctx, bucket, objectPath, minio.GetObjectOptions{})
}

// GetObjectSize obtiene el tamaño de un objeto.
func (m *MinIO) GetObjectSize(ctx context.Context, bucket, objectPath string) (int64, error) {
	info, err := m.client.StatObject(ctx, bucket, objectPath, minio.StatObjectOptions{})
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

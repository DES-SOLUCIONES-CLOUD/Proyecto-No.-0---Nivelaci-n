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

// MinIO gestiona la conexión al almacenamiento de objetos desde el worker.
type MinIO struct {
	client      *minio.Client
	BucketOrig  string
	BucketBundl string
}

// NewMinIO crea y verifica la conexión a MinIO.
func NewMinIO(endpoint, accessKey, secretKey, bucketOrig, bucketBundl string, useSSL bool) (*MinIO, error) {
	var client *minio.Client
	var err error

	for i := 0; i < 10; i++ {
		client, err = minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: useSSL,
		})
		if err == nil {
			_, err = client.ListBuckets(context.Background())
			if err == nil {
				break
			}
		}
		log.Printf("[MinIO Worker] Intento %d/10: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a MinIO: %w", err)
	}

	log.Println("[MinIO Worker] Conexión establecida")
	return &MinIO{client: client, BucketOrig: bucketOrig, BucketBundl: bucketBundl}, nil
}

// DownloadOriginal descarga el documento original desde MinIO.
func (m *MinIO) DownloadOriginal(ctx context.Context, objectPath string) ([]byte, error) {
	obj, err := m.client.GetObject(ctx, m.BucketOrig, objectPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error descargando original: %w", err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("error leyendo original: %w", err)
	}
	return data, nil
}

// UploadBundle sube el bundle ZIP al bucket de bundles en MinIO.
func (m *MinIO) UploadBundle(ctx context.Context, objectPath string, data []byte) error {
	_, err := m.client.PutObject(ctx, m.BucketBundl, objectPath, 
		newBytesReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/zip"},
	)
	return err
}

// bytesReader implementa io.Reader sobre un slice de bytes.
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

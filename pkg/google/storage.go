package google

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"time"

	cloudStorage "cloud.google.com/go/storage"
	"github.com/kinkando/pharma-sheet-service/pkg/generator"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/option"
)

const (
	GoogleCloudStorageBaseURL = "https://storage.googleapis.com"
)

type Storage interface {
	UploadFile(ctx context.Context, file *multipart.FileHeader, directory string) (string, error)
	RemoveFile(ctx context.Context, path string) error
	SetPublic(ctx context.Context, fileName string) error
	GetPublicUrl(fileName string) string
	Shutdown()
}

type storage struct {
	ctx        context.Context
	client     *cloudStorage.Client
	config     *jwt.Config
	bucketName string
	expireTime time.Duration
}

func NewStorage(credential []byte, bucketName string, expireTime time.Duration) Storage {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("cloud storage: connecting")

	client, err := cloudStorage.NewClient(ctx, option.WithCredentialsJSON(credential))
	if err != nil {
		logger.Fatal(err)
	}

	conf, err := google.JWTConfigFromJSON(credential)
	if err != nil {
		logger.Fatalf("unable to load jwt config from json credential: %+v", err)
	}

	logger.Info("cloud storage: connected")

	return &storage{
		ctx:        ctx,
		client:     client,
		config:     conf,
		bucketName: bucketName,
		expireTime: expireTime,
	}
}

func (s *storage) UploadFile(ctx context.Context, file *multipart.FileHeader, directory string) (objectName string, err error) {
	fileExt := filepath.Ext(file.Filename)

	src, err := file.Open()
	if err != nil {
		err = fmt.Errorf("error read file %s", err)
		return
	}
	defer src.Close()

	uuid := generator.UUID()
	objectName = directory + "/" + uuid + fileExt
	bucketName := s.bucketName

	wc := s.client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	if _, err = io.Copy(wc, src); err != nil {
		return "", fmt.Errorf("error copy file %s", err)
	}
	if err := wc.Close(); err != nil {
		return "", err
	}

	return objectName, nil
}

func (s *storage) RemoveFile(ctx context.Context, path string) error {
	o := s.client.Bucket(s.bucketName).Object(path)
	if err := o.Delete(ctx); err != nil {
		return fmt.Errorf("Object(%q).Delete: %v", path, err)
	}

	return nil
}

func (s *storage) getSignedUrl(objectName string) (fileURL string, err error) {
	opts := &cloudStorage.SignedURLOptions{
		Scheme:         cloudStorage.SigningSchemeV4,
		Method:         "GET",
		GoogleAccessID: s.config.Email,
		PrivateKey:     s.config.PrivateKey,
		Expires:        time.Now().Add(s.expireTime),
	}
	fileURL, err = cloudStorage.SignedURL(s.bucketName, objectName, opts)
	if err != nil {
		return fileURL, fmt.Errorf("storage.SignedURL: %v", err)
	}
	return fileURL, nil
}

func (s *storage) SetPublic(ctx context.Context, fileName string) error {
	acl := s.client.Bucket(s.bucketName).Object(fileName).ACL()
	if err := acl.Set(ctx, cloudStorage.AllUsers, cloudStorage.RoleReader); err != nil {
		return fmt.Errorf("ACLHandle.Set: %v", err)
	}

	return nil
}

func (s *storage) GetPublicUrl(fileName string) (publicUrl string) {
	return GoogleCloudStorageBaseURL + "/" + s.bucketName + "/" + fileName
}

func (s *storage) Shutdown() {
	logger.Info("cloud storage: shutting down")
	if err := s.client.Close(); err != nil {
		logger.Errorf("cloud storage: close: %s", err.Error())
		return
	}
	logger.Info("cloud storage: shut down")
}

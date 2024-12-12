package google

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	cloudStorage "cloud.google.com/go/storage"
	"github.com/kinkando/pharma-sheet/pkg/generator"
	"github.com/kinkando/pharma-sheet/pkg/logger"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	GoogleCloudStorageBaseURL = "https://storage.googleapis.com"
)

type Storage interface {
	UploadFileWithName(file *multipart.FileHeader, objectName string) (string, error)
	UploadFile(file *multipart.FileHeader, serviceName string) (string, string, error)
	UploadFileByte(fileByte []byte, pathFile string, contentType string) error
	GetFileUrl(fileName string) (string, error)
	RemoveFile(fileName string) error
	CopyFile(dstFolder, srcObject string) (string, error)
	SetPublic(fileName string) error
	GetPublicUrl(fileName string) string
	GetFileList(bucketName *string) (fileName []string, err error)
	GetMetadata(fileName string) (*cloudStorage.ObjectAttrs, error)
	GetObjectName(publicURL string) string
	Shutdown()
}

type storage struct {
	ctx        context.Context
	client     *cloudStorage.Client
	config     *jwt.Config
	bucketName string
	expireTime time.Duration
}

func NewStorage(credential []byte, bucketName string, expireTime time.Duration) (Storage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("cloud storage: connecting")

	client, err := cloudStorage.NewClient(ctx, option.WithCredentialsJSON(credential))
	if err != nil {
		return nil, err
	}

	conf, err := google.JWTConfigFromJSON(credential)
	if err != nil {
		return nil, fmt.Errorf("unable to load jwt config from json credential: %+v", err)
	}

	logger.Info("cloud storage: connected")

	return &storage{
		ctx:        ctx,
		client:     client,
		config:     conf,
		bucketName: bucketName,
		expireTime: expireTime,
	}, nil
}

func (s *storage) UploadFileByte(fileByte []byte, pathFileName string, contentType string) error {
	ctx := s.ctx
	wc := s.client.Bucket(s.bucketName).Object(pathFileName).NewWriter(ctx)
	wc.ContentType = contentType
	_, err := wc.Write(fileByte)
	if err != nil {
		return fmt.Errorf("write file to GCP Storage %+v", err)
	}

	err = wc.Close()
	if err != nil {
		return fmt.Errorf("close write file to GCP Storage %+v", err)
	}

	return err
}

func (s *storage) UploadFileWithName(file *multipart.FileHeader, objectName string) (fileURL string, err error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("error read file %s", err)
	}
	defer src.Close()

	bucketName := s.bucketName
	ctx := s.ctx

	wc := s.client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	if _, err = io.Copy(wc, src); err != nil {
		return "", fmt.Errorf("error copy file %s", err)
	}
	if err := wc.Close(); err != nil {
		return "", err
	}

	fileURL, err = s.getSignedUrl(objectName)
	if err != nil {
		return fileURL, err
	}

	return fileURL, nil
}

func (s *storage) UploadFile(file *multipart.FileHeader, serviceName string) (objectName string, fileURL string, err error) {
	fileExt := filepath.Ext(file.Filename)

	src, err := file.Open()
	if err != nil {
		err = fmt.Errorf("error read file %s", err)
		return
	}
	defer src.Close()

	uuid := generator.UUID()
	objectName = serviceName + "/" + uuid + fileExt
	bucketName := s.bucketName
	ctx := s.ctx

	wc := s.client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	if _, err = io.Copy(wc, src); err != nil {
		return "", "", fmt.Errorf("error copy file %s", err)
	}
	if err := wc.Close(); err != nil {
		return "", "", err
	}

	fileURL, err = s.getSignedUrl(objectName)
	if err != nil {
		return objectName, fileURL, err
	}

	return objectName, fileURL, nil
}

func (s *storage) GetFileUrl(fileName string) (fileURL string, err error) {
	if fileName == "" {
		return "", nil
	}

	fileURL, err = s.getSignedUrl(fileName)

	if err != nil {
		return fileURL, err
	}
	return fileURL, nil

}

func (s *storage) RemoveFile(fileName string) error {
	ctx := s.ctx
	o := s.client.Bucket(s.bucketName).Object(fileName)
	if err := o.Delete(ctx); err != nil {
		return fmt.Errorf("Object(%q).Delete: %v", fileName, err)
	}

	return nil
}

func (s *storage) CopyFile(dstFolder, srcObject string) (string, error) {
	ctx := s.ctx
	fileName := strings.Split(srcObject, "/")
	dstObject := dstFolder + "/" + fileName[1]
	bucketName := s.bucketName

	src := s.client.Bucket(bucketName).Object(srcObject)
	dst := s.client.Bucket(bucketName).Object(dstObject)

	if _, err := dst.CopierFrom(src).Run(ctx); err != nil {
		return "", fmt.Errorf("Object(%q).CopierFrom(%q).Run: %v", dstObject, srcObject, err)
	}

	return dstObject, nil
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

func (s *storage) SetPublic(fileName string) error {
	ctx := s.ctx
	bucketName := s.bucketName
	acl := s.client.Bucket(bucketName).Object(fileName).ACL()
	if err := acl.Set(ctx, cloudStorage.AllUsers, cloudStorage.RoleReader); err != nil {
		return fmt.Errorf("ACLHandle.Set: %v", err)
	}

	return nil
}

func (s *storage) GetPublicUrl(fileName string) (publicUrl string) {
	return GoogleCloudStorageBaseURL + "/" + s.bucketName + "/" + fileName
}

func (s *storage) GetFileList(bucketName *string) (fileName []string, err error) {
	ctx := s.ctx
	var bucket string

	if bucketName == nil || *bucketName == "" {
		bucket = s.bucketName
	} else {
		bucket = *bucketName
	}

	it := s.client.Bucket(bucket).Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fileName, fmt.Errorf("unable to get file: %s", err.Error())
		}
		fileName = append(fileName, attrs.Name)
	}

	return fileName, nil
}

func (s *storage) GetMetadata(fileName string) (*cloudStorage.ObjectAttrs, error) {
	ctx := s.ctx
	bucketName := s.bucketName

	attrs, err := s.client.Bucket(bucketName).Object(fileName).Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("Object(%q).Attrs: %v", fileName, err)
	}

	return attrs, nil
}

func (s *storage) GetObjectName(publicURL string) string {
	return strings.ReplaceAll(publicURL, GoogleCloudStorageBaseURL+"/"+s.bucketName+"/", "")
}

func (s *storage) Shutdown() {
	logger.Info("cloud storage: shutting down")
	if err := s.client.Close(); err != nil {
		logger.Errorf("cloud storage: close: %s", err.Error())
		return
	}
	logger.Info("cloud storage: shut down")
}

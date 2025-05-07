package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"sync"
	"time"

	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	MimeTypeFolder = "application/vnd.google-apps.folder"
)

type File struct {
	Metadata *Metadata
	Data     []byte
}

type Metadata struct {
	ContentType string
}

type ListFile struct {
	FolderID       string
	Query          string
	PageSize       int64
	PageToken      string
	Fields         []googleapi.Field
	OrderBy        string
	IsListAllFiles bool
}

type ServiceAccountCredential struct {
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
}

type Drive interface {
	Get(ctx context.Context, fileID string) (*File, error)
	List(ctx context.Context, req ListFile) ([]*drive.File, error)
	Upload(ctx context.Context, directory, fileName string, data []byte) (string, error)
	UploadMultipart(ctx context.Context, directory string, file *multipart.FileHeader) (string, error)
	Delete(ctx context.Context, fileID string) error
	PublicURL(ctx context.Context, fileID string) string
	MoveFromRoot(ctx context.Context, fileID, folderID string) error
	GetParentIDByDirectory(ctx context.Context, parentID, directory string) (string, error)
}

type GoogleDrive struct {
	client       *drive.Service
	rootFolderID string
	mutex        sync.RWMutex
}

func NewGoogleDrive(serviceAccount []byte, rootFolderID string) Drive {
	credential := new(ServiceAccountCredential)
	err := json.Unmarshal(serviceAccount, credential)
	if err != nil {
		logger.Fatalf("googledrive.NewGoogleDrive: json.Unmarshal: %w", err)
	}

	config := &jwt.Config{
		Email:      credential.ClientEmail,
		PrivateKey: []byte(credential.PrivateKey),
		Scopes:     []string{drive.DriveScope},
		TokenURL:   google.JWTTokenURL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := config.Client(ctx)
	service, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		logger.Fatalf("googledrive.NewGoogleDrive: %w", err)
	}

	return &GoogleDrive{client: service, rootFolderID: rootFolderID}
}

func (ggd *GoogleDrive) Get(ctx context.Context, fileID string) (*File, error) {
	res, err := ggd.client.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}
	contentType := res.Header.Get("Content-Type")

	return &File{
		Metadata: &Metadata{
			ContentType: contentType,
		},
		Data: data,
	}, nil
}

func (ggd *GoogleDrive) List(ctx context.Context, req ListFile) ([]*drive.File, error) {
	if req.PageSize <= 0 {
		req.PageSize = 100
	}
	if req.Fields == nil {
		req.Fields = []googleapi.Field{"nextPageToken", "files(id, name, mimeType, kind, fileExtension, size, createdTime, modifiedTime, parents)"}
	}
	if req.OrderBy == "" {
		req.OrderBy = "name asc"
	}

	query := req.Query
	if req.FolderID != "" {
		query = strings.TrimPrefix(query+fmt.Sprintf(" and '%s' in parents", req.FolderID), " and ")
	}

	var files []*drive.File
	pageToken := req.PageToken
	for isRemaining := true; isRemaining; isRemaining = pageToken != "" {
		fileList, err := ggd.client.Files.
			List().
			PageToken(pageToken).
			PageSize(req.PageSize).
			Fields(req.Fields...).
			OrderBy(req.OrderBy).
			Context(ctx).
			Q(query).
			Do()
		if err != nil {
			return nil, fmt.Errorf("google: sheet: List: unable to list files: %v", err)
		}

		pageToken = fileList.NextPageToken
		files = append(files, fileList.Files...)

		if !req.IsListAllFiles {
			break
		}
	}
	return files, nil
}

func (ggd *GoogleDrive) Upload(ctx context.Context, directory, fileName string, data []byte) (string, error) {
	parentID, err := ggd.GetParentIDByDirectory(ctx, ggd.rootFolderID, directory)
	if err != nil {
		return "", fmt.Errorf("failed to get parent id by directory: %s", err.Error())
	}

	file := &drive.File{
		Name:    fileName,
		Parents: []string{parentID},
	}

	result, err := ggd.client.Files.Create(file).Context(ctx).Media(bytes.NewBuffer(data)).Fields("*").Do()
	if err != nil {
		return "", fmt.Errorf("failed to write data: %w", err)
	}

	return result.Id, nil
}

func (ggd *GoogleDrive) UploadMultipart(ctx context.Context, directory string, file *multipart.FileHeader) (string, error) {
	parentID, err := ggd.GetParentIDByDirectory(ctx, ggd.rootFolderID, directory)
	if err != nil {
		return "", fmt.Errorf("failed to get parent id by directory: %s", err.Error())
	}

	fileMetadata := &drive.File{
		Name:    file.Filename,
		Parents: []string{parentID},
	}

	f, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	result, err := ggd.client.Files.Create(fileMetadata).Context(ctx).Media(f).Fields("*").Do()
	if err != nil {
		return "", fmt.Errorf("failed to write data: %w", err)
	}

	return result.Id, nil
}

func (ggd *GoogleDrive) Delete(ctx context.Context, fileID string) error {
	err := ggd.client.Files.Delete(fileID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (ggd *GoogleDrive) MoveFromRoot(ctx context.Context, fileID, folderID string) error {
	_, err := ggd.client.Files.Update(fileID, nil).AddParents(folderID).RemoveParents("root").Do()
	return err
}

func (ggd *GoogleDrive) GetParentIDByDirectory(ctx context.Context, parentID, directory string) (string, error) {
	ggd.mutex.Lock()

	var err error
	isFound := true
	directories := strings.Split(directory, "/")
	for _, dir := range directories {
		if isFound {
			query := fmt.Sprintf("mimeType = '%s' AND name = '%s'", MimeTypeFolder, dir)
			if parentID != "" {
				query += fmt.Sprintf(" AND '%s' in parents", parentID)
			}
			files, err := ggd.list(ctx, query, 1)
			if err != nil {
				return "", err
			}
			if len(files) > 0 {
				parentID = files[0].Id
				continue
			}
			isFound = false
		}

		parentID, err = ggd.createDirectory(ctx, parentID, dir)
		if err != nil {
			return "", err
		}
	}

	ggd.mutex.Unlock()
	return parentID, nil
}

func (ggd *GoogleDrive) list(ctx context.Context, query string, pageSize int64) (files []*drive.File, err error) {
	field := "nextPageToken, files(id, name, mimeType, kind, fileExtension, fullFileExtension, size, thumbnailLink, createdTime, modifiedTime, parents)"
	if pageSize <= 0 {
		pageSize = 100
	}

	var pageToken string
	for isRemaining := true; isRemaining; isRemaining = pageToken != "" {
		list := ggd.client.Files.List().Context(ctx)

		if pageToken != "" {
			list = list.PageToken(pageToken)
		}

		if query != "" {
			list = list.Q(query)
		}

		fileList, err := list.
			Fields(googleapi.Field(field)).
			PageSize(pageSize).
			Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list file: %s", err.Error())
		}

		pageToken = fileList.NextPageToken
		files = append(files, fileList.Files...)
	}

	return files, nil
}

func (ggd *GoogleDrive) createDirectory(ctx context.Context, parentID, name string) (string, error) {
	file := &drive.File{
		Name:     name,
		Parents:  []string{parentID},
		MimeType: MimeTypeFolder,
	}

	file, err := ggd.client.Files.Create(file).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %s", err.Error())
	}

	return file.Id, nil
}

func (ggd *GoogleDrive) PublicURL(ctx context.Context, fileID string) string {
	return fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID)
}

func FileID(url string) string {
	return strings.TrimSuffix(strings.TrimPrefix(url, "https://drive.google.com/file/d/"), "/view")
}

package service

import (
	"context"

	"firebase.google.com/go/auth"
	"github.com/google/uuid"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/google"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
	"github.com/kinkando/pharma-sheet-service/repository"
)

type User interface {
	GetUserInfo(ctx context.Context) (model.User, error)
}

type user struct {
	userRepository repository.User
	firebaseAuthen *auth.Client
	storage        google.Storage
}

func NewUserService(
	userRepository repository.User,
	firebaseAuthen *auth.Client,
	storage google.Storage,
) User {
	return &user{
		userRepository: userRepository,
		firebaseAuthen: firebaseAuthen,
		storage:        storage,
	}
}

func (s *user) GetUserInfo(ctx context.Context) (user model.User, err error) {
	userProfile, err := profile.UseProfile(ctx)
	if err != nil {
		return
	}

	userInfo, err := s.userRepository.GetUser(ctx, genmodel.Users{UserID: uuid.MustParse(userProfile.UserID)})
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	var imageURL *string
	if userInfo.ImageURL != nil {
		url, err := s.storage.GetUrl(*userInfo.ImageURL)
		if err != nil {
			logger.Context(ctx).Error(err)
			return user, err
		}
		imageURL = &url
	}

	user = model.User{
		UserID:      userInfo.UserID.String(),
		FirebaseUID: userInfo.FirebaseUID,
		Email:       userInfo.Email,
		ImageURL:    imageURL,
		DisplayName: userInfo.DisplayName,
	}

	return user, nil
}

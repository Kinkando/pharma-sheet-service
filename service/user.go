package service

import (
	"context"
	"fmt"

	"firebase.google.com/go/auth"
	"github.com/google/uuid"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
	"github.com/kinkando/pharma-sheet-service/repository"
)

type User interface {
	GetUserInfo(ctx context.Context) (model.User, error)
}

type user struct {
	userRepository repository.User
	firebseAuthen  *auth.Client
}

func NewUserService(
	userRepository repository.User,
	firebseAuthen *auth.Client,
) User {
	return &user{
		userRepository: userRepository,
		firebseAuthen:  firebseAuthen,
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

	if userInfo.FirebaseUID == nil {
		err = fmt.Errorf("user is not found in firebase authen")
		logger.Context(ctx).Error(err)
		return
	}

	authUser, err := s.firebseAuthen.GetUser(ctx, *userInfo.FirebaseUID)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	user = model.User{
		UserID:      userInfo.UserID.String(),
		FirebaseUID: userInfo.FirebaseUID,
		Email:       userInfo.Email,
		ImageURL:    authUser.PhotoURL,
		DisplayName: authUser.DisplayName,
	}

	return user, nil
}

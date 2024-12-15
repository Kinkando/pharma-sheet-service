package service

import (
	"context"
	"errors"
	"fmt"

	"firebase.google.com/go/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	genmodel "github.com/kinkando/pharma-sheet-service/.gen/pharma_sheet/public/model"
	"github.com/kinkando/pharma-sheet-service/model"
	"github.com/kinkando/pharma-sheet-service/pkg/logger"
	"github.com/kinkando/pharma-sheet-service/pkg/profile"
	"github.com/kinkando/pharma-sheet-service/repository"
)

type Authen interface {
	VerifyToken(ctx context.Context, idToken string) (model.JWT, error)
	RefreshToken(ctx context.Context, refreshToken string) (model.JWT, error)
	RevokeToken(ctx context.Context, jwt string) error
}

type authen struct {
	userRepository  repository.User
	cacheRepository repository.Cache
	jwtService      JWTService
	firebaseAuthen  *auth.Client
}

func NewAuthenService(
	userRepository repository.User,
	cacheRepository repository.Cache,
	jwtService JWTService,
	firebaseAuthen *auth.Client,
) Authen {
	return &authen{
		userRepository:  userRepository,
		cacheRepository: cacheRepository,
		jwtService:      jwtService,
		firebaseAuthen:  firebaseAuthen,
	}
}

func (s *authen) VerifyToken(ctx context.Context, idToken string) (model.JWT, error) {
	token, err := s.firebaseAuthen.VerifyIDTokenAndCheckRevoked(ctx, idToken)
	if err != nil {
		logger.Context(ctx).Error(err)
		return model.JWT{}, err
	}

	emails, ok := token.Firebase.Identities["email"].([]any)
	if !ok || len(emails) == 0 {
		err = fmt.Errorf("unable to get email from id token")
		logger.Context(ctx).Error(err)
		return model.JWT{}, err
	}

	jwt, err := s.createToken(ctx, genmodel.Users{FirebaseUID: &token.UID, Email: fmt.Sprintf("%+v", emails[0])})
	if err != nil {
		logger.Context(ctx).Error(err)
		return model.JWT{}, err
	}

	return jwt, nil
}

func (s *authen) RefreshToken(ctx context.Context, refreshToken string) (model.JWT, error) {
	refreshClaim, err := s.jwtService.DecodeRefreshToken(ctx, refreshToken)
	if err != nil {
		logger.Context(ctx).Error(err)
		return model.JWT{}, err
	}

	found, err := s.cacheRepository.ExistsToken(ctx, refreshClaim.UserID, refreshClaim.SessionID, profile.Refresh)
	if err != nil {
		logger.Context(ctx).Error(err)
		return model.JWT{}, err
	}

	if !found {
		err = fmt.Errorf("refreshToken is not found")
		logger.Context(ctx).Error(err)
		return model.JWT{}, err
	}

	jwt, err := s.createToken(ctx, genmodel.Users{UserID: uuid.MustParse(refreshClaim.UserID)})
	if err != nil {
		logger.Context(ctx).Error(err)
		return model.JWT{}, err
	}

	if err = s.cacheRepository.DeleteToken(ctx, refreshClaim.UserID, refreshClaim.SessionID, profile.Access); err != nil {
		logger.Context(ctx).Error(err)
		return model.JWT{}, err
	}

	if err = s.cacheRepository.DeleteToken(ctx, refreshClaim.UserID, refreshClaim.SessionID, profile.Refresh); err != nil {
		logger.Context(ctx).Error(err)
		return model.JWT{}, err
	}

	return jwt, nil
}

func (s *authen) RevokeToken(ctx context.Context, jwt string) error {
	accessToken, err := s.jwtService.DecodeAccessToken(ctx, jwt)
	if err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	if err := s.cacheRepository.DeleteToken(ctx, accessToken.UserID, accessToken.SessionID, profile.Access); err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	if err := s.cacheRepository.DeleteToken(ctx, accessToken.UserID, accessToken.SessionID, profile.Refresh); err != nil {
		logger.Context(ctx).Error(err)
		return err
	}

	return nil
}

func (s *authen) createToken(ctx context.Context, userReq genmodel.Users) (jwt model.JWT, err error) {
	user, err := s.userRepository.GetUser(ctx, userReq)
	if errors.Is(err, pgx.ErrNoRows) {
		userID, err := s.userRepository.CreateUser(ctx, userReq)
		if err != nil {
			logger.Context(ctx).Error(err)
			return jwt, err
		}
		user = userReq
		user.UserID = uuid.MustParse(userID)

	} else if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	if user.FirebaseUID == nil {
		err = s.userRepository.UpdateUser(ctx, genmodel.Users{UserID: userReq.UserID, FirebaseUID: userReq.FirebaseUID})
		if err != nil {
			logger.Context(ctx).Error(err)
			return jwt, err
		}
		user.FirebaseUID = userReq.FirebaseUID
	}

	accessToken, refreshToken := s.jwtService.EncodeJWT(ctx, user.UserID.String())

	jwt.AccessToken, err = s.jwtService.SignedJWT(ctx, accessToken)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	jwt.RefreshToken, err = s.jwtService.SignedJWT(ctx, refreshToken)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	err = s.cacheRepository.CreateAccessToken(ctx, accessToken)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	err = s.cacheRepository.CreateRefreshToken(ctx, refreshToken)
	if err != nil {
		logger.Context(ctx).Error(err)
		return
	}

	return
}

package profile

import (
	"context"
	"fmt"
)

type Role string

const ProfileKey = "profile"

const (
	Admin Role = "admin"
	User  Role = "user"
)

type Profile struct {
	UserID string `json:"userID"`
	Role   Role   `json:"role"`
}

func UseProfile(ctx context.Context) (Profile, error) {
	profile, ok := ctx.Value(ProfileKey).(Profile)
	if !ok {
		return Profile{}, fmt.Errorf(`unable to retrieve profile from context`)
	}
	return profile, nil
}

func UseAdminProfile(ctx context.Context) (Profile, error) {
	profile, err := UseProfile(ctx)
	if err != nil {
		return Profile{}, err
	}

	if profile.Role != Admin {
		return Profile{}, fmt.Errorf(`unable to retrieve admin profile from context`)
	}

	return profile, nil
}

func UseUserProfile(ctx context.Context) (Profile, error) {
	profile, err := UseProfile(ctx)
	if err != nil {
		return Profile{}, err
	}

	if profile.Role != User {
		return Profile{}, fmt.Errorf(`unable to retrieve user profile from context`)
	}

	return profile, nil
}

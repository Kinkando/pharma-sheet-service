package profile

import (
	"context"
	"fmt"
)

const ProfileKey = "profile"

type Profile struct {
	UserID string `json:"userID"`
}

func UseProfile(ctx context.Context) (Profile, error) {
	profile, ok := ctx.Value(ProfileKey).(Profile)
	if !ok {
		return Profile{}, fmt.Errorf(`unable to retrieve profile from context`)
	}
	return profile, nil
}

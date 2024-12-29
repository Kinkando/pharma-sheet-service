package model

import "mime/multipart"

type User struct {
	UserID      string  `json:"userID"`
	FirebaseUID *string `json:"-"`
	Email       string  `json:"email"`
	ImageURL    *string `json:"imageURL,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
}

type UpdateUserRequest struct {
	ProfileImage *multipart.FileHeader `form:"profileImage"`
	DisplayName  *string               `form:"displayName"`
}

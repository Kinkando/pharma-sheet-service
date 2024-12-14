package model

type User struct {
	UserID      string  `json:"userID"`
	FirebaseUID *string `json:"-"`
	Email       string  `json:"email"`
	ImageURL    string  `json:"imageURL,omitempty"`
	DisplayName string  `json:"displayName,omitempty"`
}

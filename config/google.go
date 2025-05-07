package config

import "time"

type GoogleConfig struct {
	FirebaseCredential string        `env:"FIREBASE_CREDENTIAL,required"`
	Storage            StorageConfig `envPrefix:"STORAGE_"`
	DriveRootFolderID  string        `env:"DRIVE_ROOT_FOLDER_ID,required"`
}

type StorageConfig struct {
	BucketName  string        `env:"BUCKET_NAME,required"`
	ExpiredTime time.Duration `env:"EXPIRED_TIME"`
}

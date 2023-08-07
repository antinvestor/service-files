package config

import "github.com/pitabwire/frame"

type FilesConfig struct {
	frame.ConfigurationDefault
	NotificationServiceURI string `default:"127.0.0.1:7020" envconfig:"NOTIFICATION_SERVICE_URI"`

	StorageProvider            string `default:"LOCAL" envconfig:"STORAGE_PROVIDER"`
	EnvStorageEncryptionPhrase string `default:"AES256Key-XihgT047PgfrbYZJB4Rf2K" envconfig:"ENCRYPTION_PHRASE"`

	FileAccessServerUrl string `default:"" envconfig:"FILE_ACCESS_SERVER_URL"`

	QueueFileSyncURL  string `default:"mem://file_model_sync" envconfig:"QUEUE_FILE_SYNC_URL"`
	QueueFileSyncName string `default:"file_model_sync" envconfig:"QUEUE_FILE_SYNC_NAME"`

	QueueFileAuditSyncURL  string `default:"mem://file_audit_model_sync" envconfig:"QUEUE_FILE_AUDIT_SYNC_URL"`
	QueueFileAuditSyncName string `default:"file_audit_model_sync" envconfig:"QUEUE_FILE_AUDIT_SYNC_NAME"`

	CsrfSecret string `default:"" envconfig:"CSRF_SECRET"`

	ProviderGcsPrivateBucket string `default:"" envconfig:"GCS_PRIVATE_BUCKET"`
	ProviderGcsPublicBucket  string `default:"" envconfig:"GCS_PUBLIC_BUCKET"`
	ProviderS3PrivateBucket  string `default:"" envconfig:"S3_PRIVATE_BUCKET"`
	ProviderS3PublicBucket   string `default:"" envconfig:"S3_PUBLIC_BUCKET"`
	ProviderS3Endpoint       string `default:"" envconfig:"S3_ENDPOINT"`
	ProviderS3Region         string `default:"" envconfig:"S3_REGION"`
	ProviderS3Secret         string `default:"" envconfig:"S3_SECRET"`
	ProviderS3Token          string `default:"" envconfig:"S3_TOKEN"`
	ProviderS3AccessKeyId    string `default:"" envconfig:"S3_ACCESS_KEY_ID"`
}

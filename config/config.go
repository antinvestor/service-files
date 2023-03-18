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
}

package config

import (
	"github.com/pitabwire/frame/config"
)

// A Path on the filesystem.
type Path string

// FileSizeBytes is a file size in bytes
type FileSizeBytes int64

// ThumbnailSize contains a single thumbnail size configuration
type ThumbnailSize struct {
	// Maximum width of the thumbnail image
	Width int `yaml:"width"`
	// Maximum height of the thumbnail image
	Height int `yaml:"height"`
	// ResizeMethod is one of crop or scale.
	// crop scales to fill the requested dimensions and crops the excess.
	// scale scales to fit the requested dimensions and one dimension may be smaller than requested.
	ResizeMethod string `yaml:"method,omitempty"`
}

// DefaultMaxFileSizeBytes defines the default file size allowed in transfers
var DefaultMaxFileSizeBytes = FileSizeBytes(10485760)

//func (c *MediaAPI) Defaults(opts DefaultOpts) {
//	c.MaxFileSizeBytes = DefaultMaxFileSizeBytes
//	c.MaxThumbnailGenerators = 10
//	c.ThumbnailSizes = []ThumbnailSize{
//		{
//			Width:        32,
//			Height:       32,
//			ResizeMethod: "crop",
//		},
//		{
//			Width:        96,
//			Height:       96,
//			ResizeMethod: "crop",
//		},
//		{
//			Width:        640,
//			Height:       480,
//			ResizeMethod: "scale",
//		},
//	}
//	c.Database.ConnectionString = opts.DatabaseConnectionStr
//	c.BasePath = "/tmp/media_store"
//
//}

type FilesConfig struct {
	config.ConfigurationDefault
	NotificationServiceURI string `envDefault:"127.0.0.1:7020" env:"NOTIFICATION_SERVICE_URI"`

	StorageProvider            string `envDefault:"LOCAL" env:"STORAGE_PROVIDER"`
	EnvStorageEncryptionPhrase string `envDefault:"AES256Key-XihgT047PgfrbYZJB4Rf2K" env:"ENCRYPTION_PHRASE"`

	FileAccessServerUrl string `envDefault:"" env:"FILE_ACCESS_SERVER_URL"`

	QueueThumbnailsGenerateURL  string `envDefault:"mem://thumbnails_generate" env:"QUEUE_THUMBNAILS_GENERATE_URL"`
	QueueThumbnailsGenerateName string `envDefault:"thumbnails_generate" env:"QUEUE_THUMBNAILS_GENERATE_NAME"`

	CsrfSecret string `envDefault:"" env:"CSRF_SECRET"`

	ProviderGcsPrivateBucket  string `envDefault:"" env:"GCS_PRIVATE_BUCKET"`
	ProviderGcsPublicBucket   string `envDefault:"" env:"GCS_PUBLIC_BUCKET"`
	ProviderS3PrivateBucket   string `envDefault:"" env:"S3_PRIVATE_BUCKET"`
	ProviderS3PublicBucket    string `envDefault:"" env:"S3_PUBLIC_BUCKET"`
	ProviderS3Endpoint        string `envDefault:"" env:"S3_ENDPOINT"`
	ProviderS3Region          string `envDefault:"" env:"S3_REGION"`
	ProviderS3AccessKeySecret string `envDefault:"" env:"S3_ACCESS_KEY_SECRET"`
	ProviderS3SessionToken    string `envDefault:"" env:"S3_SESSION_TOKEN"`
	ProviderS3AccessKeyId     string `envDefault:"" env:"S3_ACCESS_KEY_ID"`

	ServerName string ``

	// The base path to where the media files will be stored. May be relative or absolute.
	BasePath Path `yaml:"base_path"`

	// The absolute base path to where media files will be stored.
	AbsBasePath Path `yaml:"-"`

	// The maximum file size in bytes that is allowed to be stored on this server.
	// Note: if max_file_size_bytes is set to 0, the size is unlimited.
	// Note: if max_file_size_bytes is not set, it will default to 10485760 (10MB)
	MaxFileSizeBytes FileSizeBytes `yaml:"max_file_size_bytes,omitempty"`

	// Whether to dynamically generate thumbnails on-the-fly if the requested resolution is not already generated
	DynamicThumbnails bool `yaml:"dynamic_thumbnails"`

	// The maximum number of simultaneous thumbnail generators. default: 10
	MaxThumbnailGenerators int `yaml:"max_thumbnail_generators"`

	// A list of thumbnail sizes to be pre-generated for downloaded remote / uploaded content
	ThumbnailSizes []ThumbnailSize `yaml:"thumbnail_sizes"`
}

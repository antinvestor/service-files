package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pitabwire/frame/v2/config"
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

type FilesConfig struct {
	config.ConfigurationDefault
	NotificationServiceURI string `envDefault:"127.0.0.1:7020" env:"NOTIFICATION_SERVICE_URI"`

	StorageProvider            string `envDefault:"LOCAL" env:"STORAGE_PROVIDER"`
	EnvStorageEncryptionPhrase string `envDefault:"" env:"ENCRYPTION_PHRASE"`

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
	// Note: if max_file_size_bytes is not set (or 0), it will default to 10485760 (10MB)
	MaxFileSizeBytes FileSizeBytes `yaml:"max_file_size_bytes,omitempty" env:"MAX_FILE_SIZE_BYTES"`

	// Whether to dynamically generate thumbnails on-the-fly if the requested resolution is not already generated
	DynamicThumbnails bool `yaml:"dynamic_thumbnails" envDefault:"false" env:"DYNAMIC_THUMBNAILS"`

	// The maximum number of simultaneous thumbnail generators. default: 10
	MaxThumbnailGenerators int `yaml:"max_thumbnail_generators"`

	// Maximum allowed thumbnail width/height in pixels. Default: 2048
	MaxThumbnailDimension int `envDefault:"2048" env:"MAX_THUMBNAIL_DIMENSION"`

	// ThumbnailSizesSpec is the env form of ThumbnailSizes: a comma-separated list of
	// WIDTHxHEIGHT[:METHOD] entries, e.g. "32x32:crop,96x96:crop,640x480:scale".
	// METHOD defaults to scale. Parsed into ThumbnailSizes by Normalise.
	ThumbnailSizesSpec string `envDefault:"" env:"THUMBNAIL_SIZES"`

	// A list of thumbnail sizes to be pre-generated for downloaded remote / uploaded content
	ThumbnailSizes []ThumbnailSize `yaml:"thumbnail_sizes"`
}

// DefaultThumbnailSizes are pre-generated when THUMBNAIL_SIZES is unset.
func DefaultThumbnailSizes() []ThumbnailSize {
	return []ThumbnailSize{
		{Width: 32, Height: 32, ResizeMethod: "crop"},
		{Width: 96, Height: 96, ResizeMethod: "crop"},
		{Width: 640, Height: 480, ResizeMethod: "scale"},
	}
}

// ParseThumbnailSizes parses a THUMBNAIL_SIZES specification such as
// "32x32:crop,96x96:crop,640x480:scale". Entries are trimmed, empty entries are skipped and
// the resize method defaults to scale.
func ParseThumbnailSizes(spec string) ([]ThumbnailSize, error) {
	var sizes []ThumbnailSize
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		dims, method, _ := strings.Cut(entry, ":")
		method = strings.ToLower(strings.TrimSpace(method))
		if method == "" {
			method = "scale"
		}
		if method != "crop" && method != "scale" {
			return nil, fmt.Errorf("thumbnail size %q: method must be crop or scale", entry)
		}
		wStr, hStr, ok := strings.Cut(strings.ToLower(strings.TrimSpace(dims)), "x")
		if !ok {
			return nil, fmt.Errorf("thumbnail size %q: expected WIDTHxHEIGHT[:METHOD]", entry)
		}
		w, err := strconv.Atoi(strings.TrimSpace(wStr))
		if err != nil || w <= 0 {
			return nil, fmt.Errorf("thumbnail size %q: invalid width", entry)
		}
		h, err := strconv.Atoi(strings.TrimSpace(hStr))
		if err != nil || h <= 0 {
			return nil, fmt.Errorf("thumbnail size %q: invalid height", entry)
		}
		sizes = append(sizes, ThumbnailSize{Width: w, Height: h, ResizeMethod: method})
	}
	return sizes, nil
}

// Normalise applies defaults and validates configuration values.
func (c *FilesConfig) Normalise() error {
	if c.MaxFileSizeBytes == 0 {
		c.MaxFileSizeBytes = DefaultMaxFileSizeBytes
	}

	if c.MaxThumbnailGenerators == 0 {
		c.MaxThumbnailGenerators = 10
	}

	if c.MaxThumbnailDimension == 0 {
		c.MaxThumbnailDimension = 2048
	}

	if c.ThumbnailSizesSpec != "" {
		sizes, err := ParseThumbnailSizes(c.ThumbnailSizesSpec)
		if err != nil {
			return fmt.Errorf("THUMBNAIL_SIZES: %w", err)
		}
		if len(sizes) > 0 {
			c.ThumbnailSizes = sizes
		}
	}
	if len(c.ThumbnailSizes) == 0 {
		c.ThumbnailSizes = DefaultThumbnailSizes()
	}
	for _, size := range c.ThumbnailSizes {
		if size.Width > c.MaxThumbnailDimension || size.Height > c.MaxThumbnailDimension {
			return fmt.Errorf("thumbnail size %dx%d exceeds MAX_THUMBNAIL_DIMENSION (%d)",
				size.Width, size.Height, c.MaxThumbnailDimension)
		}
	}

	if c.BasePath == "" {
		c.BasePath = "/tmp/media_store"
	}

	abs, err := filepath.Abs(string(c.BasePath))
	if err != nil {
		return err
	}
	c.AbsBasePath = Path(abs)

	return nil
}

package storage

import (
	"bytes"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
)

type ProviderWasabi struct {
	name string
	privateBucket string
	publicBucket string

	wasabiEndpoint    string
	wasabiAccessKeyID string
	wasabiSecret      string
	wasabiToken       string
	wasabiRegion      string
	wasabiSession     *session.Session
}

func (provider *ProviderWasabi) Name()string   {
	return provider.name
}

func (provider *ProviderWasabi) PublicBucket()string   {
	return provider.publicBucket
}

func (provider *ProviderWasabi) PrivateBucket()string   {
	return provider.privateBucket
}

func (provider *ProviderWasabi) Init(ctx context.Context) (interface{}, error) {

	if provider.wasabiSession == nil {
		s3Config := &aws.Config{
			Credentials:      credentials.NewStaticCredentials(provider.wasabiAccessKeyID, provider.wasabiSecret, provider.wasabiToken),
			Endpoint:         aws.String(provider.wasabiEndpoint),
			Region:           aws.String(provider.wasabiRegion),
			S3ForcePathStyle: aws.Bool(true),
		}

		sess, err := session.NewSession(s3Config)
		if err != nil {
			return  nil, err
		}

		provider.wasabiSession = sess

	}

	return provider.wasabiSession, nil

}

func (provider *ProviderWasabi) UploadFile(ctx context.Context, bucket string, pathName string, extension string, contents []byte) (string, error) {

	bucketObject := aws.String(bucket)
	key := aws.String(pathName)

	sessionObj, err := provider.Init(ctx)
	if err != nil {
		return "", err
	}

	s3session, ok := sessionObj.(*session.Session)
	if !ok{
		return "", errors.New("could not cast client object to S3 Object")
	}

	wasabiClient := s3.New(s3session)

	// Upload a new object "wasabi-testobject" with the string "Wasabi Hot storage"
	result, err := wasabiClient.PutObject(&s3.PutObjectInput{
		Body:   bytes.NewReader(contents),
		Bucket: bucketObject,
		Key:    key,
	})

	if err != nil {
		return "", err
	}

	return result.String(), nil

}

func (provider *ProviderWasabi) DownloadFile(ctx context.Context, bucket string, pathName string, extension string) ([]byte, error) {

	bucketObject := aws.String(bucket)
	key := aws.String(pathName)

	sessionObj, err := provider.Init(ctx)
	if err != nil {
		return nil, err
	}

	s3session, ok := sessionObj.(*session.Session)
	if !ok{
		return nil, errors.New("could not cast client object to S3 Object")
	}

	wasabiClient := s3.New(s3session)

	//Get Object
	result, err := wasabiClient.GetObject(&s3.GetObjectInput{
		Bucket: bucketObject,
		Key:    key,
	})
	if err != nil {
		return nil, err
	}

	defer result.Body.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, result.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil

}

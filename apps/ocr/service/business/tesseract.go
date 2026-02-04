package business

import (
	"context"
	"os"

	gosseract2 "github.com/otiai10/gosseract/v2"
)

type tesseract struct {
}

func (ts *tesseract) Recognise(ctx context.Context, image *os.File) (string, error) {

	localClient := gosseract2.NewClient()
	defer func() { _ = localClient.Close() }()

	err := localClient.SetImage(image.Name())
	if err != nil {
		return "", err
	}

	result, err := localClient.Text()
	if err != nil {
		return "", err
	}

	return result, err

}

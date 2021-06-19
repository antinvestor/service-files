package business

import (
	"fmt"
	"github.com/antinvestor/files/openapi"
	"github.com/antinvestor/files/service/models"
)

func FileToApi(fileAccessServer string, file *models.File) openapi.File {

	fileUrl := fmt.Sprintf("%s/%s", fileAccessServer, file.ID)

	return openapi.File{
		Id:             file.ID,
		Name:           file.Name,
		Public:         file.Public,
		GroupId:        file.GroupID,
		SubscriptionId: file.SubscriptionID,
		Url:            fileUrl,
	}
}


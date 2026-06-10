package service

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	mcpopenapi "github.com/QuantumNous/new-api/pkg/mcp/openapi"
)

var ErrMCPOpenAPIBinaryObjectNotFound = errors.New("openapi binary object not found")

type MCPOpenAPIBinaryDownloadParams struct {
	UserId   int
	IsAdmin  bool
	ObjectId string
}

type MCPOpenAPIBinaryDownload struct {
	Object  model.MCPOpenAPIBinaryObject
	Content []byte
}

func DownloadMCPOpenAPIBinaryObject(ctx context.Context, params MCPOpenAPIBinaryDownloadParams) (MCPOpenAPIBinaryDownload, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if params.UserId <= 0 {
		return MCPOpenAPIBinaryDownload{}, ErrMCPOpenAPIBinaryObjectNotFound
	}
	registryObject, err := model.GetMCPOpenAPIBinaryObjectByObjectId(params.ObjectId)
	if err != nil {
		return MCPOpenAPIBinaryDownload{}, ErrMCPOpenAPIBinaryObjectNotFound
	}
	if registryObject.ExpiresAt > 0 && registryObject.ExpiresAt < common.GetTimestamp() {
		return MCPOpenAPIBinaryDownload{}, ErrMCPOpenAPIBinaryObjectNotFound
	}
	if !params.IsAdmin && registryObject.UserId != params.UserId {
		return MCPOpenAPIBinaryDownload{}, ErrMCPOpenAPIBinaryObjectNotFound
	}
	storeObject, content, err := mcpopenapi.LoadBinaryObjectWithContext(ctx, registryObject.ObjectId)
	if err != nil {
		return MCPOpenAPIBinaryDownload{}, err
	}
	if registryObject.ContentType == "" {
		registryObject.ContentType = storeObject.ContentType
	}
	if registryObject.Filename == "" {
		registryObject.Filename = storeObject.Filename
	}
	if err := model.TouchMCPOpenAPIBinaryObjectDownload(registryObject.ObjectId); err != nil {
		return MCPOpenAPIBinaryDownload{}, err
	}
	return MCPOpenAPIBinaryDownload{
		Object:  *registryObject,
		Content: content,
	}, nil
}

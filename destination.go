// Copyright © 2024 Meroxa, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vectorstore

//go:generate paramgen -output=paramgen_dest.go DestinationConfig

import (
	"context"
	"errors"
	"fmt"

	"github.com/conduitio/conduit-commons/config"
	"github.com/conduitio/conduit-commons/opencdc"
	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/sashabaranov/go-openai"
)

type Destination struct {
	sdk.UnimplementedDestination

	config DestinationConfig
	client *openai.Client
}

//go:generate paramgen -output=paramgen_dest.go DestinationConfig

type DestinationConfig struct {
	// APIKey is the OpenAI api key to use for the api client.
	APIKey string `json:"api_key" validate:"required"`

	// VectorStoreID is the id of the vector store to write records into.
	VectorStoreID string `json:"vector_store_id" validate:"required"`
}

func NewDestination() sdk.Destination {
	return sdk.DestinationWithMiddleware(&Destination{}, sdk.DefaultDestinationMiddleware()...)
}

func (d *Destination) Parameters() config.Parameters {
	return d.config.Parameters()
}

func (d *Destination) Configure(ctx context.Context, cfg config.Config) error {
	sdk.Logger(ctx).Info().Msg("configuring destination...")
	err := sdk.Util.ParseConfig(ctx, cfg, &d.config, NewDestination().Parameters())
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}

func (d *Destination) Open(ctx context.Context) error {
	d.client = openai.NewClient(d.config.APIKey)

	// check that the passed api key is valid, so that we ensure that the api
	// calls have no auth errors.

	if _, err := d.client.ListModels(ctx); err != nil {
		return fmt.Errorf("failed to validate api key: %w", err)
	}

	sdk.Logger(ctx).Info().Msg("api key is valid")

	_, err := d.client.RetrieveVectorStore(ctx, d.config.VectorStoreID)
	if err != nil {
		return fmt.Errorf("failed to retrieve vector store %s: %w", d.config.VectorStoreID, err)
	}

	return nil
}

func (d *Destination) Write(ctx context.Context, recs []opencdc.Record) (int, error) {
	for i, rec := range recs {
		switch rec.Operation {
		case opencdc.OperationCreate, opencdc.OperationSnapshot:
			if err := d.createFile(ctx, rec); err != nil {
				return i, err
			}
		case opencdc.OperationDelete:
			if err := d.deleteFile(ctx, rec); err != nil {
				return i, err
			}
		case opencdc.OperationUpdate:
			if err := d.updateFile(ctx, rec); err != nil {
				return i, err
			}
		}
	}

	return 0, nil
}

func (d *Destination) createFile(ctx context.Context, rec opencdc.Record) error {
	filename := string(rec.Key.Bytes())
	filebs := rec.Payload.After.Bytes()
	f, err := d.client.CreateFileBytes(ctx, openai.FileBytesRequest{
		Name:    filename,
		Bytes:   filebs,
		Purpose: openai.PurposeAssistants,
	})
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	sdk.Logger(ctx).Info().Str("filename", filename).Msg("Created file")

	_, err = d.client.CreateVectorStoreFile(ctx,
		d.config.VectorStoreID, openai.VectorStoreFileRequest{FileID: f.ID})
	if err != nil {
		return fmt.Errorf(
			"failed to add file %s to vector store %s: %w",
			f.FileName, d.config.VectorStoreID, err)
	}

	sdk.Logger(ctx).Info().
		Str("filename", filename).
		Str("vector_store_id", d.config.VectorStoreID).
		Msg("Added file to vector store")

	return nil
}

var ErrFileNotFound = errors.New("file not found")

func (d *Destination) deleteFile(ctx context.Context, rec opencdc.Record) error {
	filename := string(rec.Key.Bytes())

	files, err := d.client.ListFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	var fileID string
	for _, file := range files.Files {
		if file.FileName == filename {
			fileID = file.ID
		}
	}
	if fileID == "" {
		return ErrFileNotFound
	}

	err = d.client.DeleteVectorStoreFile(ctx, d.config.VectorStoreID, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete file from vector store: %w", err)
	}

	sdk.Logger(ctx).Info().Str("filename", filename).Msg("Deleted file from vector store")

	if err = d.client.DeleteFile(ctx, fileID); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	sdk.Logger(ctx).Info().Str("filename", filename).Msg("Deleted file")

	return nil
}

func (d *Destination) updateFile(ctx context.Context, rec opencdc.Record) error {
	// OpenAI doesn't provide a way to update the uploaded file, so we need to
	// delete it and upload it again

	filename := string(rec.Key.Bytes())

	if err := d.deleteFile(ctx, rec); err != nil {
		return fmt.Errorf("failed to delete file while updating: %w", err)
	}

	sdk.Logger(ctx).Info().Str("filename", filename).Msg("Deleted file while updating")

	if err := d.createFile(ctx, rec); err != nil {
		return fmt.Errorf("failed to create file while updating: %w", err)
	}

	sdk.Logger(ctx).Info().Str("filename", filename).Msg("Created file while updating")

	return nil
}

func (d *Destination) Teardown(_ context.Context) error {
	return nil
}

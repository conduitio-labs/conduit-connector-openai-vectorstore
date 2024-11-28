package vectorstore

//go:generate paramgen -output=paramgen_dest.go DestinationConfig

import (
	"context"
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

	vectorStoreID string
}

//go:generate paramgen -output=paramgen_dest.go DestinationConfig

type DestinationConfig struct {

	// APIKey is the openai api key to use for the api client.
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
	sdk.Logger(ctx).Info().Msg("Configuring Destination...")
	err := sdk.Util.ParseConfig(ctx, cfg, &d.config, NewDestination().Parameters())
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}

func (d *Destination) Open(ctx context.Context) error {
	d.client = openai.NewClient(d.config.APIKey)

	// check that the passed api key is valid
	if _, err := d.client.GetModel(ctx, "gpt-4"); err != nil {
		return fmt.Errorf("failed to validate api key: %w", err)
	}


	return nil
}

func (d *Destination) Write(ctx context.Context, recs []opencdc.Record) (int, error) {

	for i, rec := range recs {
		filename := string(rec.Key.Bytes())
		filebs := rec.Payload.After.Bytes()

		f, err := d.client.CreateFileBytes(ctx, openai.FileBytesRequest{
			Name:    filename,
			Bytes:   filebs,
			Purpose: openai.PurposeAssistants,
		})
		if err != nil {
			return i, fmt.Errorf("failed to create file: %w", err)
		}

		_, err = d.client.CreateVectorStoreFile(ctx,
			d.config.VectorStoreID, openai.VectorStoreFileRequest{FileID: f.ID})
		if err != nil {
			return i, fmt.Errorf("failed to add file %s to vector store %s: %w", f.FileName, d.vectorStoreID, err)
		}
	}

	return 0, nil
}

func (d *Destination) Teardown(_ context.Context) error {
	return nil
}

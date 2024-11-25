package openai-vectorstore_test

import (
	"context"
	"testing"

	openai-vectorstore "github.com/conduitio-labs/conduit-connector-openai-vectorstore"
	"github.com/matryer/is"
)

func TestTeardownSource_NoOpen(t *testing.T) {
	is := is.New(t)
	con := openai-vectorstore.NewSource()
	err := con.Teardown(context.Background())
	is.NoErr(err)
}

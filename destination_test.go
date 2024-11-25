package openai-vectorstore_test

import (
	"context"
	"testing"

	openai-vectorstore "github.com/conduitio-labs/conduit-connector-openai-vectorstore"
	"github.com/matryer/is"
)

func TestTeardown_NoOpen(t *testing.T) {
	is := is.New(t)
	con := openai-vectorstore.NewDestination()
	err := con.Teardown(context.Background())
	is.NoErr(err)
}

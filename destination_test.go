package vectorstore

import (
	"context"
	"log"
	"os"
	"testing"

	_ "embed"

	"github.com/conduitio/conduit-commons/config"
	"github.com/conduitio/conduit-commons/opencdc"
	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/matryer/is"
	"github.com/sashabaranov/go-openai"
)

var getOpenAIApiKey = envGetter("OPENAI_API_KEY")

func testDestination(ctx context.Context, t *testing.T, vectorStoreID string) sdk.Destination {
	is := is.New(t)
	dest := NewDestination()

	err := dest.Configure(ctx, config.Config{
		DestinationConfigApiKey:        getOpenAIApiKey(t),
		DestinationConfigVectorStoreId: vectorStoreID,
	})
	is.NoErr(err)

	err = dest.Open(ctx)
	is.NoErr(err)

	t.Cleanup(func() { is.NoErr(dest.Teardown(ctx)) })

	return dest
}

func testClient(t *testing.T) *openai.Client {
	return openai.NewClient(getOpenAIApiKey(t))
}

func createTestVectorStore(ctx context.Context, t *testing.T) string {
	is := is.New(t)
	client := testClient(t)

	vectorStore, err := client.CreateVectorStore(ctx, openai.VectorStoreRequest{
		Name: "conduit test vector store",
		ExpiresAfter: &openai.VectorStoreExpires{
			Days:   1,
			Anchor: "last_active_at",
		},
	})
	is.NoErr(err)

	return vectorStore.ID
}

func cleanupAllTestData() {
	apikey := os.Getenv("OPENAI_API_KEY")
	if apikey == "" {
		// nothing to delete, as there's no way something was written without an api key
		return
	}

	ctx := context.Background()
	client := openai.NewClient(apikey)
	files, err := client.ListFiles(ctx)
	if err != nil {
		log.Fatalf("failed to list files: %v", err)
	}

	for _, file := range files.Files {
		if err = client.DeleteFile(ctx, file.ID); err != nil {
			log.Fatalf("failed to delete file %s: %v", file.FileName, err)
		}
	}

	log.Println("successfully deleted all files")

	vectorStores, err := client.ListVectorStores(ctx, openai.Pagination{})
	if err != nil {
		log.Fatalf("failed to list vector stores: %v", err)
	}

	for _, vectorStore := range vectorStores.VectorStores {
		_, err := client.DeleteVectorStore(ctx, vectorStore.ID)
		if err != nil {
			log.Fatalf("failed to delete vector store %s: %v", vectorStore.ID, err)
		}
	}

	log.Println("successfully deleted all vector stores")
}

func TestTeardown_NoOpen(t *testing.T) {
	is := is.New(t)
	con := NewDestination()
	err := con.Teardown(context.Background())
	is.NoErr(err)
}

func TestWrite_Create(t *testing.T) {
	ctx := context.Background()
	is := is.New(t)
	var err error

	vectorStoreID := createTestVectorStore(ctx, t)

	dest := testDestination(ctx, t, vectorStoreID)

	filename := "test filename.txt"

	_, err = dest.Write(ctx, []opencdc.Record{
		sdk.Util.Source.NewRecordCreate(
			nil,
			nil,
			opencdc.RawData(filename),
			opencdc.RawData("aaa"),
		),
	})
	is.NoErr(err)

	assertFileWritten(ctx, t, filename, vectorStoreID)
}

func assertFileWritten(ctx context.Context, t *testing.T, filename, vectorStoreID string) {
	client := testClient(t)
	is := is.New(t)

	// assert that file exists
	files, err := client.ListFiles(ctx)
	is.NoErr(err)

	var fileID string
	for _, file := range files.Files {
		if file.FileName == filename {
			fileID = file.ID
		}
	}
	if fileID == "" {
		t.Fatalf("failed to find filename %s", filename)
	}

	vectorStoreFile, err := client.RetrieveVectorStoreFile(ctx, vectorStoreID, fileID)
	is.NoErr(err)

	is.Equal(vectorStoreFile.Object, filename) // unexpected vector store file object
}

func TestWrite_Snapshot(t *testing.T) {
}

func TestWrite_Update(t *testing.T) {
}

func TestWrite_Delete(t *testing.T) {
}

func envGetter(key string) func(t *testing.T) string {
	return func(t *testing.T) string {
		env := os.Getenv(key)
		if env == "" {
			t.Fatalf("missing env var %s", key)
		}

		return env
	}
}

func TestMain(m *testing.M) {
	m.Run()

	// cleanupAllTestData()
}

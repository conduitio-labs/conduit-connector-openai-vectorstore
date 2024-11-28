package vectorstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	_ "embed"

	"github.com/conduitio/conduit-commons/config"
	"github.com/conduitio/conduit-commons/opencdc"
	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/google/uuid"
	"github.com/matryer/is"
	"github.com/rs/zerolog"
	"github.com/sashabaranov/go-openai"
)

var getOpenAIApiKey = envGetter("OPENAI_API_KEY")

func TestTeardown_NoOpen(t *testing.T) {
	is := is.New(t)
	con := NewDestination()
	err := con.Teardown(context.Background())
	is.NoErr(err)
}

func TestWrite_Create(t *testing.T) {
	ctx := testContext(t)
	is := is.New(t)
	var err error

	vectorStoreID := createTestVectorStore(ctx, t)

	dest := testDestination(ctx, t, vectorStoreID)

	file := newTestFile()
	rec := file.recCreate()

	_, err = dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)

	assertFileWritten(ctx, t, file, vectorStoreID)
}

func TestWrite_Snapshot(t *testing.T) {
	ctx := testContext(t)
	is := is.New(t)
	var err error

	vectorStoreID := createTestVectorStore(ctx, t)

	dest := testDestination(ctx, t, vectorStoreID)

	file := newTestFile()
	rec := file.recSnapshot()

	_, err = dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)

	assertFileWritten(ctx, t, file, vectorStoreID)
}

func TestWrite_Update(t *testing.T) {
	ctx := testContext(t)
	is := is.New(t)
	var err error

	vectorStoreID := createTestVectorStore(ctx, t)

	dest := testDestination(ctx, t, vectorStoreID)

	file := newTestFile()
	rec := file.recSnapshot()

	_, err = dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)

	file.contents = "bbb"
	rec = file.recUpdate()

	_, err = dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)

	assertFileWritten(ctx, t, file, vectorStoreID)
}

func TestWrite_Delete(t *testing.T) {
	ctx := testContext(t)
	is := is.New(t)
	var err error

	vectorStoreID := createTestVectorStore(ctx, t)

	dest := testDestination(ctx, t, vectorStoreID)

	file := newTestFile()
	rec := file.recSnapshot()

	_, err = dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)

	rec = opencdc.Record{
		Operation: opencdc.OperationDelete,
		Key:       opencdc.RawData(file.name),
	}

	_, err = dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)

	assertDeletedFile(ctx, t, file.name)
}

func TestMain(m *testing.M) {
	m.Run()

	cleanupAllTestData()
}

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

func testContext(t *testing.T) context.Context {
	var writer io.Writer
	if t != nil {
		writer = zerolog.ConsoleWriter{
			Out:        zerolog.NewTestWriter(t),
			PartsOrder: []string{"level", "message"},
		}
	} else {
		writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			PartsOrder: []string{"level", "message"},
		}
	}

	traceLog := os.Getenv("TRACE") == "true"
	level := zerolog.InfoLevel
	if traceLog {
		level = zerolog.TraceLevel
	}
	logger := zerolog.New(writer).Level(level)

	return logger.WithContext(context.Background())
}

func createTestVectorStore(ctx context.Context, t *testing.T) string {
	is := is.New(t)
	client := testClient(t)

	name := fmt.Sprint("test vector store", uuid.NewString()[0:8])

	vectorStore, err := client.CreateVectorStore(ctx, openai.VectorStoreRequest{
		Name: name,
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

	ctx := testContext(nil)

	sdk.Logger(ctx).Info().Msg("cleaning up test files...")

	client := openai.NewClient(apikey)
	files, err := client.ListFiles(ctx)
	if err != nil {
		sdk.Logger(ctx).Fatal().Err(err).Msg("failed to list files")
	}

	for _, file := range files.Files {
		if err = client.DeleteFile(ctx, file.ID); err != nil {
			sdk.Logger(ctx).Fatal().Err(err).Str("file", file.FileName).Msg("failed to delete file")
		}
	}

	sdk.Logger(ctx).Info().Msg("successfully cleaned test files files")
	sdk.Logger(ctx).Info().Msg("deleting vector stores...")

	vectorStores, err := client.ListVectorStores(ctx, openai.Pagination{})
	if err != nil {
		sdk.Logger(ctx).Fatal().Err(err).Msg("failed to list vector stores")
	}

	for _, vectorStore := range vectorStores.VectorStores {
		_, err := client.DeleteVectorStore(ctx, vectorStore.ID)
		if err != nil {
			sdk.Logger(ctx).Fatal().Err(err).Str("vectorStoreID", vectorStore.ID).Msg("failed to delete vector store")
		}
	}

	sdk.Logger(ctx).Info().Msg("successfully deleted test vector stores")
}

type testFile struct {
	name     string
	contents string
}

func newTestFile() testFile {
	return testFile{
		name:     fmt.Sprint("test-", uuid.NewString()[0:8], ".txt"),
		contents: "test contents",
	}
}

func (f testFile) recCreate() opencdc.Record {
	return opencdc.Record{
		Position:  nil,
		Operation: opencdc.OperationCreate,
		Metadata:  nil,
		Key:       opencdc.RawData(f.name),
		Payload: opencdc.Change{
			After: opencdc.RawData(f.contents),
		},
	}
}

func (f testFile) recSnapshot() opencdc.Record {
	return opencdc.Record{
		Position:  nil,
		Operation: opencdc.OperationSnapshot,
		Metadata:  nil,
		Key:       opencdc.RawData(f.name),
		Payload: opencdc.Change{
			After: opencdc.RawData(f.contents),
		},
	}
}

func (f testFile) recUpdate() opencdc.Record {
	return opencdc.Record{
		Position:  nil,
		Operation: opencdc.OperationUpdate,
		Metadata:  nil,
		Key:       opencdc.RawData(f.name),
		Payload: opencdc.Change{
			After: opencdc.RawData(f.contents),
		},
	}
}

func assertFileWritten(
	ctx context.Context, t *testing.T, writtenFile testFile, vectorStoreID string) {
	client := testClient(t)
	is := is.New(t)

	// assert that file exists
	files, err := client.ListFiles(ctx)
	is.NoErr(err)

	var fileID string
	for _, openaiFile := range files.Files {
		if openaiFile.FileName == writtenFile.name {
			fileID = openaiFile.ID
		}
	}
	if fileID == "" {
		t.Fatalf("failed to find filename %s", writtenFile.name)
	}

	// OpenAI doesn't provide a way to get the file contents, so we can't assert
	// that the file contents are correct. In theory we could create an AI agent and
	// check that the file that it has access to has the proper contents.

	_, err = client.RetrieveVectorStoreFile(ctx, vectorStoreID, fileID)
	is.NoErr(err)
}

func assertDeletedFile(ctx context.Context, t *testing.T, filename string) {
	client := testClient(t)
	is := is.New(t)

	files, err := client.ListFiles(ctx)
	is.NoErr(err)

	for _, openaiFile := range files.Files {
		if openaiFile.FileName == filename {
			t.Fatalf("file %s still exists", filename)
		}
	}
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

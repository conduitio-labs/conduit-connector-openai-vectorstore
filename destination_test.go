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

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

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

	written, err := dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)
	is.Equal(written, 1)

	assertFileWrittenAndUnique(ctx, t, file, vectorStoreID)
}

func TestWrite_MultipleWritesWithSameKeyDontDuplicate(t *testing.T) {
	ctx := testContext(t)
	is := is.New(t)
	var err error

	vectorStoreID := createTestVectorStore(ctx, t)

	dest := testDestination(ctx, t, vectorStoreID)

	file1, file2 := newTestFile(), newTestFile()
	file2.name = file1.name

	written, err := dest.Write(ctx, []opencdc.Record{file1.recSnapshot(), file2.recCreate()})
	is.NoErr(err)
	is.Equal(written, 2)

	assertFileWrittenAndUnique(ctx, t, file2, vectorStoreID)
}

func TestWrite_Snapshot(t *testing.T) {
	ctx := testContext(t)
	is := is.New(t)
	var err error

	vectorStoreID := createTestVectorStore(ctx, t)

	dest := testDestination(ctx, t, vectorStoreID)

	file := newTestFile()
	rec := file.recSnapshot()

	written, err := dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)
	is.Equal(written, 1)

	assertFileWrittenAndUnique(ctx, t, file, vectorStoreID)
}

func TestWrite_Update(t *testing.T) {
	ctx := testContext(t)
	is := is.New(t)
	var err error

	vectorStoreID := createTestVectorStore(ctx, t)

	dest := testDestination(ctx, t, vectorStoreID)

	file := newTestFile()
	rec := file.recSnapshot()

	written, err := dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)
	is.Equal(written, 1)

	file.contents = "updated contents"
	rec = file.recUpdate()

	written, err = dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)
	is.Equal(written, 1)

	assertFileWrittenAndUnique(ctx, t, file, vectorStoreID)
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

	written, err := dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)
	is.Equal(written, 1)

	assertDeletedFile(ctx, t, file.name)
}

func TestMain(m *testing.M) {
	m.Run()

	// it's much easier to clean up all test data after all tests have run, so
	// that we don't worry of left over test data on each test.
	// Assuming each test creates unique data, ofc.
	cleanupAllTestData()
}

func testDestination(ctx context.Context, t *testing.T, vectorStoreID string) sdk.Destination {
	t.Helper()
	is := is.New(t)
	dest := NewDestination()

	cfg := config.Config{
		"api_key":         getOpenAIApiKey(t),
		"vector_store_id": vectorStoreID,
	}

	err := sdk.Util.ParseConfig(ctx, cfg, dest.Config(), Connector.NewSpecification().DestinationParams)
	is.NoErr(err)

	err = dest.Open(ctx)
	is.NoErr(err)

	t.Cleanup(func() { is.NoErr(dest.Teardown(ctx)) })

	return dest
}

func testClient(t *testing.T) *openai.Client {
	t.Helper()
	return openai.NewClient(getOpenAIApiKey(t))
}

//nolint:thelper // no t usage
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
	t.Helper()
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

	// By default, the list files call has a limit of 10k files, so there should
	// never be a need to paginate.

	for _, file := range files.Files {
		if err = client.DeleteFile(ctx, file.ID); err != nil {
			sdk.Logger(ctx).Fatal().Err(err).Str("file", file.FileName).Msg("failed to delete file")
		}
	}

	sdk.Logger(ctx).Info().Msg("successfully cleaned test files")
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

func assertFileWrittenAndUnique(
	ctx context.Context, t *testing.T, writtenFile testFile, vectorStoreID string,
) {
	t.Helper()
	client := testClient(t)
	is := is.New(t)

	// assert that file exists
	files, err := client.ListFiles(ctx)
	is.NoErr(err)

	var fileID string
	for _, openaiFile := range files.Files {
		if openaiFile.FileName == writtenFile.name {
			if fileID != "" {
				t.Fatalf("found multiple files with name %s", writtenFile.name)
			}
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
	t.Helper()
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
		t.Helper()
		env := os.Getenv(key)
		if env == "" {
			t.Fatalf("missing env var %s", key)
		}

		return env
	}
}

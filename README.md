# Conduit destination connector for OpenAI vector store

The OpenAI vector store connector is one of [Conduit](https://github.com/ConduitIO/conduit) standalone plugins. It provides a destination connector for [OpenAI vector stores](https://platform.openai.com/docs/api-reference/vector-stores).

# How is the record written

The connector will read the whole record and try to upload the `.Payload.After` bytes into OpenAI. There's no support for partial uploads at the moment, each record represents a single file. If two records with the same key are received, a single file will be written.

In order to process an update record, we delete the file and create it, which might be a bit slow. Updates then must include the whole file in the record.

Files are written with `assistants` as [purpose](https://platform.openai.com/docs/api-reference/files/create#files-create-purpose).

Deletes work as expected, deleting the file from the vector store and deleting the file itself from OpenAI.

## Configuration

| name              | description                        | required | example                       |
| ----------------- | ---------------------------------- | -------- | ----------------------------- |
| `api_key`         | The openai api key                 | true     | `sk-proj-....`                |
| `vector_store_id` | The vector store to write the file | true     | `vs_JXxAUJr0iYJMjDW2Mx6yl431` |

# How to run the tests

DO NOT run the tests with a production API key. All data is destroyed after running all tests. Rather:

- Create a new OpenAI project
- Create a new api key. It must have write permissions.
- Create an `.env` file just like the `.env.sample` with the api key
- Run `make test` to start the tests.

# Limitations

- Because the filename is the key, the connector doesn't support duplicated files, contrary to OpenAI.

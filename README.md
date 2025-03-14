# Conduit destination connector for OpenAI vector store

<!-- readmegen:description -->
The OpenAI vector store connector is one of [Conduit](https://github.com/ConduitIO/conduit) standalone plugins. It provides a destination connector for [OpenAI vector stores](https://platform.openai.com/docs/api-reference/vector-stores).

# How is the record written

The connector will read the whole record and try to upload the `.Payload.After` bytes into OpenAI. There's no support for partial uploads at the moment, each record represents a single file. If two records with the same key are received, a single file will be written.

In order to process an update record, we delete the file and create it, which might be a bit slow. Updates then must include the whole file in the record.

Files are written with `assistants` with [purpose](https://platform.openai.com/docs/api-reference/files/create#files-create-purpose) as  part of the request body.

Deletes work as expected, deleting the file from the vector store and deleting the file itself from OpenAI.
<!-- /readmegen:description -->

## Configuration

<!-- readmegen:destination.parameters.yaml -->
```yaml
version: 2.2
pipelines:
  - id: example
    status: running
    connectors:
      - id: example
        plugin: "openai-vectorstore"
        settings:
          # APIKey is the OpenAI api key to use for the api client.
          # Type: string
          # Required: yes
          api_key: ""
          # VectorStoreID is the id of the vector store to write records into.
          # Type: string
          # Required: yes
          vector_store_id: ""
          # Maximum delay before an incomplete batch is written to the
          # destination.
          # Type: duration
          # Required: no
          sdk.batch.delay: "0"
          # Maximum size of batch before it gets written to the destination.
          # Type: int
          # Required: no
          sdk.batch.size: "0"
          # Allow bursts of at most X records (0 or less means that bursts are
          # not limited). Only takes effect if a rate limit per second is set.
          # Note that if `sdk.batch.size` is bigger than `sdk.rate.burst`, the
          # effective batch size will be equal to `sdk.rate.burst`.
          # Type: int
          # Required: no
          sdk.rate.burst: "0"
          # Maximum number of records written per second (0 means no rate
          # limit).
          # Type: float
          # Required: no
          sdk.rate.perSecond: "0"
          # The format of the output record. See the Conduit documentation for a
          # full list of supported formats
          # (https://conduit.io/docs/using/connectors/configuration-parameters/output-format).
          # Type: string
          # Required: no
          sdk.record.format: "opencdc/json"
          # Options to configure the chosen output record format. Options are
          # normally key=value pairs separated with comma (e.g.
          # opt1=val2,opt2=val2), except for the `template` record format, where
          # options are a Go template.
          # Type: string
          # Required: no
          sdk.record.format.options: ""
          # Whether to extract and decode the record key with a schema.
          # Type: bool
          # Required: no
          sdk.schema.extract.key.enabled: "false"
          # Whether to extract and decode the record payload with a schema.
          # Type: bool
          # Required: no
          sdk.schema.extract.payload.enabled: "false"
```
<!-- /readmegen:destination.parameters.yaml -->

# How to run the tests

DO NOT run the tests with a production API key. All data is destroyed after running all tests. Rather:

- Create a new OpenAI project
- Create a new api key. It must have write permissions.
- Create an `.env` file just like the `.env.sample` with the api key
- Run `make test` to start the tests.

# Limitations

- Because the filename is the key, the connector doesn't support duplicated files, contrary to OpenAI.

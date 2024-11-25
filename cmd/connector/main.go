package main

import (
	vectorstore "github.com/conduitio-labs/conduit-connector-openai-vectorstore"
	sdk "github.com/conduitio/conduit-connector-sdk"
)

func main() {
	sdk.Serve(vectorstore.Connector)
}

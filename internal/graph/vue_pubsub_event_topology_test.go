package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestVueSFCExtractsSocketEventRole(t *testing.T) {
	extractor, err := graph.NewVueSFCExtractor()
	if err != nil {
		t.Fatal(err)
	}
	edges, err := extractor.ExtractEdges("SocketInstance.vue", []byte(`<template><div /></template>
<script>
socket.on('tradestate', (state) => setTradeState(state))
</script>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range edges {
		if edge.Metadata != nil && edge.Metadata["event_role"] == "subscribe" && edge.Metadata["topic"] == "tradestate" {
			return
		}
	}
	t.Fatalf("missing tradestate subscribe edge: %#v", edges)
}

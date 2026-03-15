package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"net/http"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	GoMemoryServer = " http://localhost:9000"
)

type GetMemoryReq struct {
	UserId    string `json:"userId" jsonschema:"userId of the user"`
	UserQuery string `json:"query" jsonschema:"topic based query about which information about the user needs to be retrieved"`
}

var client = &http.Client{Timeout: 5 * time.Second}

func GetMemory(ctx context.Context, reqs *mcp.CallToolRequest, input GetMemoryReq) (
	*mcp.CallToolResult, any, error,
) {
	// ... call your GoMemory REST API here ...

	url := "http://localhost:9000/get_memory"
	method := "POST"

	body, err := json.Marshal(input)
	if err != nil {
		return nil, nil, err
	}

	payload := bytes.NewReader(body)

	req, err := http.NewRequestWithContext(ctx, method, url, payload)

	if err != nil {
		return nil, nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, nil, fmt.Errorf("Go Memory has returned %d: %s ", res.StatusCode, body)
	}
	var memories []types.Memory

	if err := json.NewDecoder(res.Body).Decode(&memories); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}
	text := ""
	for _, mem := range memories {
		text += mem.Memory_text + "\n"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}, nil, nil
}

func main() {
	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "Go Memory",
		Version: "1.0.0",
	}, nil)

	// Add get_forecast tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_memory",
		Description: "Get memory about the user",
	}, GetMemory)

	// Run server on stdio transport
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

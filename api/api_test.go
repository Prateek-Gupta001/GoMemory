package api

import (
	"fmt"
	"testing"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/assert"
)

var sampleMessages = []types.Message{
	{Role: types.RoleUser, Content: "Hello there. How are you doing? I hope you are well."},
	{Role: types.RoleAssistant, Content: "I am doing great! Thanks for asking. What can I help you with?"},
	{Role: types.RoleUser, Content: "I need help with Go. Specifically with goroutines. And also channels."},
	{Role: types.RoleAssistant, Content: "Sure! Goroutines are lightweight threads. Channels are used to communicate between them."},
}

func TestConstructContextualQueries_EmptyMessages(t *testing.T) {
	result := ConstructContextualQueries(sampleMessages, 1000)
	assert.Equal(t, len(result), 1)
}

// Messages with empty content should be silently skipped.
func TestConstructContextualQueries_SkipsEmptyContent(t *testing.T) {
	msgs := []types.Message{
		{Role: types.RoleUser, Content: ""},
		{Role: types.RoleAssistant, Content: "   "},
		{Role: types.RoleUser, Content: "Only this sentence matters."},
	}
	result := ConstructContextualQueries(msgs, 500)
	assert.Len(t, result, 1)
	fmt.Print(result[0])
	assert.Contains(t, result[0], "Only this sentence matters")
}

// Content without punctuation should be treated as a single sentence and
// still land in a chunk without panicking.
func TestConstructContextualQueries_NoPunctuation(t *testing.T) {
	msgs := []types.Message{
		{Role: types.RoleUser, Content: "no punctuation here at all"},
	}
	result := ConstructContextualQueries(msgs, 500)
	assert.Len(t, result, 1)
	assert.Contains(t, result[0], "no punctuation here at all")
}

// A single sentence longer than charLimit must not be dropped —
// it should occupy its own oversized chunk.
func TestConstructContextualQueries_SentenceLongerThanLimit(t *testing.T) {
	msgs := []types.Message{
		{Role: types.RoleUser, Content: "This is a very long sentence that definitely exceeds the tiny char limit we set."},
	}
	result := ConstructContextualQueries(msgs, 10)
	assert.Len(t, result, 1, "oversized sentence should still be returned, not dropped")
}

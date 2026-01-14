package embed

import "github.com/Prateek-Gupta001/GoMemory/types"

type Embed interface {
	GenerateEmbeddings(user_query string) (types.Embedding, error)
}

type EmbeddingClient struct {
	EmbedServiceUrl string
}

func NewEmbeddingClient(EmbedServiceUrl string) *EmbeddingClient {
	return &EmbeddingClient{
		EmbedServiceUrl: EmbedServiceUrl,
	}
}

func (e *EmbeddingClient) GenerateEmbeddings(user_query string) (types.Embedding, error) {
	return nil, nil
}

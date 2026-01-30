package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Prateek-Gupta001/GoMemory/memory"
	"github.com/Prateek-Gupta001/GoMemory/storage"
	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/google/uuid"
)

type MemoryServer struct {
	listenAddr string
	store      storage.Storage
	memory     memory.Memory
}

func NewMemoryServer(listenAddr string, store storage.Storage, memory memory.Memory) *MemoryServer {
	return &MemoryServer{
		listenAddr: listenAddr,
		store:      store,
		memory:     memory,
	}
}

func (m *MemoryServer) Run() error {
	r := http.NewServeMux()
	r.HandleFunc("/add_memory", convertToHandleFunc(m.InsertIntoMemory))
	r.HandleFunc("/get_memory", convertToHandleFunc(m.GetMemory))
	if err := http.ListenAndServe(m.listenAddr, r); err != nil {
		slog.Error("Got this error while trying to listen and serve the http server", "error", err)
		return err
	}
	slog.Info("AI Memory is at your service Sire!")
	return nil
}

type APIError struct {
	Error   error
	Message string //don't wanna send the user/hacker at the frontend .. anything that they might wanna know ... like the error itself
	Status  int    //hence send a custom message right then and there ...
}

type MemoryInsertionResponse struct {
	ReqId string
	Msg   string
}

type apiFunc func(w http.ResponseWriter, r *http.Request) *APIError

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func convertToHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err.Error != nil {
			slog.Error("Got this error in the request handler", "error", err.Error)
			writeJSON(w, err.Status, err.Message)
		}
	}
}

func (m *MemoryServer) InsertIntoMemory(w http.ResponseWriter, r *http.Request) *APIError {
	slog.Info("------------------------------------------------NEW REQUEST------------------------------------------------")
	req := &types.InsertMemoryRequest{}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		slog.Error("Got this error while trying to decode the json body! Bad request", "error", err)
		return &APIError{
			Error:   err,
			Status:  http.StatusBadRequest,
			Message: "Request format is wrong",
		}
	}
	reqId := uuid.NewString()
	slog.Info("request Id intialised", "reqId", reqId)
	memJob := types.MemoryInsertionJob{
		Messages: req.Messages,
		ReqId:    reqId,
		UserId:   req.UserId,
	}
	err := m.memory.SumbitMemoryInsertionRequest(memJob)
	if err != nil {
		slog.Info("Got this error while trying to insert memory", "error", err)
		return &APIError{
			Error:  err,
			Status: http.StatusInternalServerError,
		}
	}
	m.store.InsertMemoryRequest(req, reqId) //Sumbit this one as well .....
	writeJSON(w, http.StatusOK, MemoryInsertionResponse{
		ReqId: reqId,
		Msg:   "Memory Insertion Job has been queued for insertion!",
	})
	return &APIError{
		Error:  nil,
		Status: http.StatusOK,
	}
}

func (m *MemoryServer) GetMemory(w http.ResponseWriter, r *http.Request) *APIError {
	var req = &types.MemoryRetrievalRequest{}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		slog.Info("Got malformed JSON here in the Get Memory request", "error", err)
		return &APIError{
			Message: "malformed JSON here in the Get Memory request",
			Error:   err,
			Status:  http.StatusBadRequest,
		}
	}
	reqId := uuid.NewString()
	req.ReqId = reqId
	if req.Messages != nil {
		slog.Info("Messages type request came in here!", "reqId", reqId)
		//TODO: Update the python grpc server ... to support asymmetric retreival ... (Sparse query: 2000 chars, Dense query: 500 characters)
		query := ConstructContextualQuery(req.Messages, 500)
		Memories, err := m.memory.GetMemories(query, req.UserId, reqId, ctx)
		if err != nil {
			slog.Error("Got this error while trying to get memories", "error", err)
			return &APIError{
				Message: "Memory Retrieval Failed!",
				Error:   err,
				Status:  http.StatusInternalServerError,
			}
		}
		writeJSON(w, http.StatusOK, Memories)

		return &APIError{
			Message: "Messages format is not currently being supported! But we are working on it",
			Error:   nil,
			Status:  200,
		}
	}
	if req.UserQuery != "" {
		slog.Info("UserQuery type request came in here!", "reqId", reqId, "userQuery", req.UserQuery)
		userQuery := req.UserQuery
		Memories, err := m.memory.GetMemories(userQuery, req.UserId, reqId, ctx)
		if err != nil {
			slog.Error("Got this error while trying to get memories", "error", err)
			return &APIError{
				Message: "Memory Retrieval Failed!",
				Error:   err,
				Status:  http.StatusInternalServerError,
			}
		}
		writeJSON(w, http.StatusOK, Memories)
	}
	return &APIError{
		Message: "Memory Retrieval was succesful",
		Error:   nil,
		Status:  http.StatusOK,
	}
}

func (m *MemoryServer) GetAllUserMemories(w http.ResponseWriter, r *http.Request) *APIError {
	req := &types.GetAllUserMemoriesRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		slog.Info("Got this error while decoding GetAllUserMemoriesRequest", "err", err)
		return &APIError{
			Message: "Bad request",
			Status:  http.StatusBadRequest,
			Error:   err,
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()
	m.memory.GetAllUserMemories(req.UserId, ctx)
	// select{
	// case <- ctx.Done():
	// 	slog.Info("Memory retrieval request timed out!", "userId", req.UserId)
	// 	return &APIError{
	// 		Message: "Retrieval Timed out",
	// 		Status: http.StatusGatewayTimeout,
	// 		Error: fmt.Errorf("Too Much Time taken!"),
	// 	}

	// }
	return &APIError{}
}

func ConstructContextualQuery(messages []types.Message, charLimit int) string {
	if len(messages) == 0 {
		return ""
	}

	var accumulatedParts []string
	currentLen := 0

	re := regexp.MustCompile(`[^.!?]+[.!?]+(\s|$)`)

	// 1. Iterate BACKWARDS through messages (Latest -> Oldest)
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		sentences := re.FindAllString(content, -1)
		if len(sentences) == 0 {
			sentences = []string{content}
		}

		var msgParts []string

		for j := len(sentences) - 1; j >= 0; j-- {
			sent := strings.TrimSpace(sentences[j])
			msgParts = append([]string{sent}, msgParts...) // Prepend to keep order within message

			currentLen += len(sent)

			// Check limit inside the sentence loop
			if currentLen >= charLimit {
				break
			}
		}

		finalMsgContent := strings.Join(msgParts, " ")

		// Prepend this block to our master list of parts
		accumulatedParts = append([]string{finalMsgContent}, accumulatedParts...)

		if currentLen >= charLimit {
			break
		}
	}

	// Join all blocks with newlines to separate turns clearly
	return strings.Join(accumulatedParts, "\n")
}

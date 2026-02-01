package api

import (
	"context"
	"encoding/json"
	"fmt"
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
	r.HandleFunc("POST /add_memory", convertToHandleFunc(m.InsertIntoMemory))
	r.HandleFunc("POST /get_memory", convertToHandleFunc(m.GetMemory))
	r.HandleFunc("GET /get_all/{id}", convertToHandleFunc(m.GetAllUserMemories))
	r.HandleFunc("GET /health", convertToHandleFunc(m.HealthCheck))
	r.HandleFunc("POST /delete_memory", convertToHandleFunc(m.DeleteUserMemory))
	slog.Info("AI Memory is at your service Sire!")
	if err := http.ListenAndServe(m.listenAddr, r); err != nil {
		slog.Error("Got this error while trying to listen and serve the http server", "error", err)
		return err
	}
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
		apiError := f(w, r)
		if apiError != nil {
			slog.Error("Got this error from an http handler func", "error", apiError.Error)
			writeJSON(w, apiError.Status, struct{ Error string }{Error: apiError.Message})
		}
	}
}

func (m *MemoryServer) HealthCheck(w http.ResponseWriter, r *http.Request) *APIError {
	slog.Info("Health check!")
	writeJSON(w, http.StatusOK, "Server is healthy!")
	return &APIError{}
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
		Messages:  req.Messages,
		ReqId:     reqId,
		UserId:    req.UserId,
		Threshold: 0.6,
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
	return nil
}

func (m *MemoryServer) GetMemory(w http.ResponseWriter, r *http.Request) *APIError {
	var req = &types.MemoryRetrievalRequest{}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		slog.Info("Got malformed JSON here in the Get Memory request", "error", err)
		return &APIError{
			Message: "Malformed JSON here in the Get Memory request",
			Error:   err,
			Status:  http.StatusBadRequest,
		}
	}
	if req.Threshold == 0 {
		req.Threshold = 0.65
	}
	reqId := uuid.NewString()
	req.ReqId = reqId
	if req.Messages != nil {
		slog.Info("Messages type request came in here!", "reqId", reqId)
		//TODO: Update the python grpc server ... to support asymmetric retreival ... (Sparse query: 2000 chars, Dense query: 500 characters)
		query := ConstructContextualQuery(req.Messages, 500)
		Memories, err := m.memory.GetMemories(query, req.UserId, reqId, req.Threshold, ctx)
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
		Memories, err := m.memory.GetMemories(userQuery, req.UserId, reqId, req.Threshold, ctx)
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
	return nil
}

func GetId(r *http.Request) (string, error) {
	id := r.PathValue("id")
	cleanId := strings.Trim(id, "\"' ")

	slog.Info("parsing id", "raw", id, "clean", cleanId)

	// 2. Validate: Check if it is empty AFTER cleaning
	if cleanId == "" {
		return "", fmt.Errorf("ID provided is empty or invalid")
	}

	return cleanId, nil
}

func (m *MemoryServer) GetAllUserMemories(w http.ResponseWriter, r *http.Request) *APIError {
	userId, err := GetId(r)
	if err != nil {
		slog.Error("Got this error while getting Id for GetAllUserMemories", "error", err)
		return &APIError{
			Error:   err,
			Message: "Bad Request",
			Status:  http.StatusBadRequest,
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	mem, err := m.memory.GetAllUserMemories(userId, ctx)
	if err != nil {
		slog.Error("Got this error while trying to get all memories of the user (in the memory agent)", "error", err, "userId", userId)
		return &APIError{
			Message: "Failed to get all user memories",
			Status:  http.StatusInternalServerError,
			Error:   err,
		}
	}
	writeJSON(w, http.StatusOK, mem)
	return nil
	// select{
	// case <- ctx.Done():
	// 	slog.Info("Memory retrieval request timed out!", "userId", req.UserId)
	// 	return &APIError{
	// 		Message: "Retrieval Timed out",
	// 		Status: http.StatusGatewayTimeout,
	// 		Error: fmt.Errorf("Too Much Time taken!"),
	// 	}

	// }
}

func (m *MemoryServer) DeleteUserMemory(w http.ResponseWriter, r *http.Request) *APIError {
	req := &types.DeleteMemoryRequest{}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return &APIError{
			Message: "Bad request",
			Error:   err,
			Status:  http.StatusBadRequest,
		}
	}

	err := m.memory.DeleteMemory(req.MemoryIds, r.Context())
	if err != nil {
		slog.Error("Got this error while trying to delete memory", "error", err, "userId", req.UserId)
		return &APIError{
			Message: "Deletion failed",
			Error:   err,
			Status:  http.StatusInternalServerError,
		}
	}
	return &APIError{
		Error:   nil,
		Message: "Memory Deletion succesful",
		Status:  http.StatusOK,
	}
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

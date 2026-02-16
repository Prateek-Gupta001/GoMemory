package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"regexp"
	"strings"
	"time"

	"github.com/Prateek-Gupta001/GoMemory/memory"
	"github.com/Prateek-Gupta001/GoMemory/storage"
	"github.com/Prateek-Gupta001/GoMemory/telemetry"
	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

func (m *MemoryServer) Run(ctx context.Context, stop context.CancelFunc) (err error) {

	defer stop()
	otelShutdown, err := telemetry.SetupOTelSDK(ctx)
	if err != nil {
		return err
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()
	srv := &http.Server{
		Addr:         m.listenAddr,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 10,
		Handler:      m.newHTTPHandler(),
	}
	srvErr := make(chan error, 1)
	go func() {
		slog.Info("Running HTTP server...")
		srvErr <- srv.ListenAndServe()
	}()

	// Wait for interruption.
	select {
	case err = <-srvErr:
		// Error when starting HTTP server.
		return err
	case <-ctx.Done():
		// Wait for first CTRL+C.
		// Stop receiving signal notifications as soon as possible.
		stop()
	}

	// When Shutdown is called, ListenAndServe immediately returns ErrServerClosed.
	slog.Info("Closing all api routes!")
	timeCtx, _ := context.WithTimeout(context.Background(), time.Second*5)
	err = srv.Shutdown(timeCtx)
	slog.Info("Stopping all currently ongoing memory jobs.")
	m.memory.StopMemoryAgent()
	slog.Info("Graceful shutdown in order!")
	return err
}

func (m *MemoryServer) newHTTPHandler() http.Handler {
	r := http.NewServeMux()
	handle := func(pattern string, handlerFunc http.HandlerFunc) {
		// "pattern" here will be "POST /add_memory", etc.
		// traceName will be passed to Jaeger as the operation name
		wrapped := otelhttp.NewHandler(handlerFunc, pattern)
		r.Handle(pattern, wrapped)
	}

	// Register routes with the helper
	handle("POST /add_memory", convertToHandleFunc(m.InsertIntoMemory))
	handle("POST /get_memory", convertToHandleFunc(m.GetMemory))
	handle("GET /get_all/{id}", convertToHandleFunc(m.GetAllUserMemories))
	handle("GET /get_core/{id}", convertToHandleFunc(m.GetCoreMemories))
	handle("GET /health", convertToHandleFunc(m.HealthCheck))
	handle("POST /delete_memory/general", convertToHandleFunc(m.DeleteGeneralMemory))
	handle("POST /delete_memory/core", convertToHandleFunc(m.DeleteCoreMemory))

	// Metrics endpoint (Standard, no wrap needed)
	r.Handle("/metrics", promhttp.Handler())

	return r
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
	return nil
}

var Tracer = otel.Tracer("Go_Memory")

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
	m.store.InsertMemoryRequest(req, reqId) //Sumbit this one as well ..... //TODO: Implement the storage stuff.
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
	ctx, span := Tracer.Start(ctx, "Memory Retrieval")
	defer span.End()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		span.RecordError(err)
		slog.Info("Got malformed JSON here in the Get Memory request", "error", err)
		return &APIError{
			Message: "Malformed JSON here in the Get Memory request",
			Error:   err,
			Status:  http.StatusBadRequest,
		}
	}
	span.SetAttributes(
		attribute.String("userId", req.UserId),
		attribute.String("reqId", req.ReqId),
	)
	if req.Threshold == 0 {
		slog.Info("threshold wasn't provided by default .. using the default value")
		req.Threshold = 0.65
	}
	reqId := uuid.NewString()
	req.ReqId = reqId
	if req.Messages != nil {
		span.SetAttributes(attribute.String("type", "messages"))
		if len(req.Messages) == 0 {
			span.RecordError(fmt.Errorf("No messages were provided"))
			return &APIError{
				Status:  http.StatusBadRequest,
				Message: "Need atleast one message",
				Error:   fmt.Errorf("len(messages) == 0"),
			}
		}
		slog.Info("Messages type request came in here!", "reqId", reqId)
		//TODO: Update the python grpc server ... to support asymmetric retreival ... (Sparse query: 2000 chars, Dense query: 500 characters)
		//TODO: Add concurrent chunking and memory retrieval in v2 of Go Memory.
		query := ConstructContextualQuery(req.Messages, 500)
		Memories, err := m.memory.GetMemories(query, req.UserId, reqId, req.Threshold, ctx)
		if err != nil {
			slog.Error("Got this error while trying to get memories", "error", err)
			span.RecordError(err)
			return &APIError{
				Message: "Memory Retrieval Failed!",
				Error:   err,
				Status:  http.StatusInternalServerError,
			}
		}
		writeJSON(w, http.StatusOK, Memories)
		return nil
	}
	if req.UserQuery != "" {
		span.SetAttributes(attribute.String("type", "userQuery"))
		slog.Info("UserQuery type request came in here!", "reqId", reqId, "userQuery", req.UserQuery)
		userQuery := req.UserQuery
		Memories, err := m.memory.GetMemories(userQuery, req.UserId, reqId, req.Threshold, ctx)
		if err != nil {
			span.RecordError(err)
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
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	ctx, span := Tracer.Start(ctx, "GetAllUserMemories")
	defer span.End()
	defer cancel()
	userId, err := GetId(r)
	if err != nil {
		span.RecordError(err)
		slog.Error("Got this error while getting Id for GetAllUserMemories", "error", err)
		return &APIError{
			Error:   err,
			Message: "Bad Request",
			Status:  http.StatusBadRequest,
		}
	}
	span.SetAttributes(attribute.String("userId", userId))

	mem, err := m.memory.GetAllUserMemories(userId, ctx)
	if err != nil {
		span.RecordError(err)
		slog.Error("Got this error while trying to get all memories of the user (in the memory agent)", "error", err, "userId", userId)
		return &APIError{
			Message: "Failed to get all user memories",
			Status:  http.StatusInternalServerError,
			Error:   err,
		}
	}
	writeJSON(w, http.StatusOK, mem)
	return nil
}

func (m *MemoryServer) GetCoreMemories(w http.ResponseWriter, r *http.Request) *APIError {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	slog.Info("Waiting for 5 seconds")
	time.Sleep(time.Second * 5)
	slog.Info("Wait over!")
	ctx, span := Tracer.Start(ctx, "GetCoreMemories")
	defer span.End()
	defer cancel()
	userId, err := GetId(r)
	if err != nil {
		slog.Error("Got this error while getting Id for GetCoreMemories", "error", err)
		span.RecordError(err)
		return &APIError{
			Error:   err,
			Message: "Bad Request",
			Status:  http.StatusBadRequest,
		}
	}
	span.SetAttributes(attribute.String("userId", userId))

	mem, err := m.memory.GetCoreMemories(userId, ctx)
	if err != nil {
		slog.Info("Got this error while trying to get the core memories of the user", "userId", userId)
		span.RecordError(err)
		return &APIError{
			Status:  http.StatusInternalServerError,
			Message: "Oops something went wrong! Please try again later!",
			Error:   err,
		}
	}
	if mem == nil {
		return &APIError{
			Status:  http.StatusOK,
			Message: "User has no core memories!",
			Error:   err,
		}

	}
	writeJSON(w, http.StatusOK, mem)
	return nil
}

func (m *MemoryServer) DeleteGeneralMemory(w http.ResponseWriter, r *http.Request) *APIError {
	//TODO: Update this endpoint to take in core memory Ids as well!
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	ctx, span := Tracer.Start(ctx, "DeleteUserMemory")
	defer span.End()
	defer cancel()
	req := &types.DeleteMemoryRequest{}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return &APIError{
			Message: "Bad request",
			Error:   err,
			Status:  http.StatusBadRequest,
		}
	}
	span.SetAttributes(attribute.String("userId", req.UserId))
	err := m.memory.DeleteMemory(req.MemoryIds, ctx)
	if err != nil {
		span.RecordError(err)
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

func (m *MemoryServer) DeleteCoreMemory(w http.ResponseWriter, r *http.Request) *APIError {
	//TODO: Update this endpoint to take in core memory Ids as well!
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	ctx, span := Tracer.Start(ctx, "DeleteCoreMemory")
	defer span.End()
	defer cancel()
	req := &types.DeleteMemoryRequest{}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return &APIError{
			Message: "Bad request",
			Error:   err,
			Status:  http.StatusBadRequest,
		}
	}
	span.SetAttributes(attribute.String("userId", req.UserId))
	err := m.memory.DeleteCoreMemory(req.MemoryIds, req.UserId, ctx)
	if err != nil {
		span.RecordError(err)
		slog.Error("Got this error while trying to delete core memory", "error", err, "userId", req.UserId)
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

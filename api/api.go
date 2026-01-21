package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

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

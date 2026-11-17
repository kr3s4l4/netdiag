package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// apiError is the standard error envelope.
type apiError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// maxBodyBytes caps JSON request bodies.
const maxBodyBytes = 1 << 20 // 1 MiB

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func (s *Server) ok(w http.ResponseWriter, v any) { writeJSON(w, http.StatusOK, v) }

func (s *Server) created(w http.ResponseWriter, v any) { writeJSON(w, http.StatusCreated, v) }

// fail writes an error and logs 5xx responses.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, status int, msg string) {
	if status >= 500 {
		s.log.Error("error de API", "ruta", r.URL.Path, "metodo", r.Method, "error", msg)
	}
	writeJSON(w, status, apiError{Error: msg})
}

func (s *Server) failCode(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	writeJSON(w, status, apiError{Error: msg, Code: code})
}

// decode reads a JSON body with a hard size limit.
func decode(r *http.Request, v any) error {
	if r.Body == nil {
		return errors.New("cuerpo vacío")
	}
	limited := io.LimitReader(r.Body, maxBodyBytes)
	dec := json.NewDecoder(limited)
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("cuerpo vacío")
		}
		return err
	}
	return nil
}

func queryString(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func queryInt(r *http.Request, key string, def int) int {
	v := queryString(r, key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func queryInt64(r *http.Request, key string) int64 {
	n, _ := strconv.ParseInt(queryString(r, key), 10, 64)
	return n
}

func queryBool(r *http.Request, key string) bool {
	v := strings.ToLower(queryString(r, key))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func pathInt64(r *http.Request, key string) (int64, error) {
	raw := r.PathValue(key)
	if raw == "" {
		return 0, errors.New("falta el identificador")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("identificador inválido")
	}
	return id, nil
}

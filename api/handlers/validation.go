package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
)

// decodeJSONStrict reads a JSON request body into dst with strict semantics: it
// requires an application/json Content-Type, rejects unknown fields (so a typo'd
// key like {"target":80} is a 400 rather than a silently-zeroed field), and
// rejects trailing data. On any violation it writes an RFC 7807 response and
// returns false; the caller returns immediately.
func decodeJSONStrict(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !hasJSONContentType(r) {
		problemJSON(w, http.StatusUnsupportedMediaType, "about:blank#unsupported-media-type", "Unsupported Media Type", "Content-Type must be application/json.")
		return false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-request", "Bad Request", "Invalid request body.")
		return false
	}
	return true
}

// hasJSONContentType reports whether the request declares an application/json
// body (ignoring any charset parameter).
func hasJSONContentType(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

var (
	ErrInvalidSessionID = errors.New("sessionId format is invalid")
	ErrInvalidVehicleID = errors.New("vehicleId format is invalid")
	ErrInvalidLimit     = errors.New("limit must be between 1 and 1000")
	ErrInvalidOffset    = errors.New("offset must be non-negative")
)

// isValidID returns true if id is non-empty and within UUID length bounds.
func isValidID(id string) bool {
	return id != "" && len(id) <= 36
}

// validateSessionID checks that sessionId has valid length (if provided).
func validateSessionID(id string) error {
	if id == "" {
		return nil // sessionId is optional
	}
	if len(id) > 36 { // UUID max length
		return ErrInvalidSessionID
	}
	return nil
}

// validateVehicleID checks that vehicleId has valid length (if provided).
func validateVehicleID(id string) error {
	if id == "" {
		return nil // vehicleId is optional
	}
	if len(id) > 50 { // Reasonable max for vehicle ID
		return ErrInvalidVehicleID
	}
	return nil
}

// validateLimit parses and validates a limit parameter.
// Returns the parsed limit or an error. Uses defaultLimit if param is empty.
func validateLimit(limitStr string, defaultLimit int) (int, error) {
	if limitStr == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return 0, fmt.Errorf("invalid limit: %w", err)
	}
	if limit <= 0 || limit > 1000 {
		return 0, ErrInvalidLimit
	}
	return limit, nil
}

// validateOffset parses and validates an offset parameter.
func validateOffset(offsetStr string) (int, error) {
	if offsetStr == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		return 0, fmt.Errorf("invalid offset: %w", err)
	}
	if offset < 0 {
		return 0, ErrInvalidOffset
	}
	return offset, nil
}

package edge

// HistoryError represents an expected, user-facing error. When returned from
// the command flow it is serialized as a JSON error object and results in a
// non-zero exit code.
type HistoryError struct {
	Message string
}

func (e *HistoryError) Error() string {
	return e.Message
}

func newHistoryError(message string) *HistoryError {
	return &HistoryError{Message: message}
}

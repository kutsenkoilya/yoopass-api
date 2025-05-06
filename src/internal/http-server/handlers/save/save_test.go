package save

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"
	resp "yoopass-api/internal/http-server/handlers/response"

	// Assuming cipher package exists and works
	// Import for UUID validation
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSecretSaver is a mock type for the SecretSaver interface
type MockSecretSaver struct {
	mock.Mock
}

func (m *MockSecretSaver) Set(key string, value []byte, ttl time.Duration) error {
	args := m.Called(key, value, ttl)
	return args.Error(0)
}

// Helper to create a JSON request body
func newJsonRequest(t *testing.T, data interface{}) *bytes.Buffer {
	t.Helper()
	body, err := json.Marshal(data)
	require.NoError(t, err)
	return bytes.NewBuffer(body)
}

// Regular expression to validate UUID format
var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Regular expression to validate the generated hex key format (assuming 32 hex chars for 16 bytes)
var keyRegex = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestSaveHandler(t *testing.T) {
	// Discard logger for cleaner test output
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})).With(slog.String("test", "save"))
	// Use slog.New(slog.NewJSONHandler(io.Discard, nil)) if you don't want any logs during tests

	testCases := []struct {
		name           string
		requestBody    *bytes.Buffer
		setupMock      func(m *MockSecretSaver)                            // Setup expectations *before* handler runs
		checkMock      func(t *testing.T, m *MockSecretSaver, req Request) // Check calls *after* handler runs
		expectedStatus int
		expectedBody   interface{}                                                                 // Can be Response, resp.Response, or map[string]interface{} for validation errors
		checkResponse  func(t *testing.T, rr *httptest.ResponseRecorder, expectedBody interface{}) // Custom checks for dynamic fields
	}{
		{
			name: "Success Save",
			requestBody: newJsonRequest(t, Request{
				Message:    "my secret message",
				Expiration: 24, // 24 hours
				OneTime:    false,
			}),
			setupMock: func(m *MockSecretSaver) {
				// Expect Set to be called with any UUID string, any byte slice, and 24h duration
				m.On("Set",
					mock.MatchedBy(func(key string) bool { return uuidRegex.MatchString(key) }), // Check key is UUID format
					mock.AnythingOfType("[]uint8"),                                              // Check value is a byte slice
					time.Duration(24)*time.Hour,                                                 // Check TTL
				).Return(nil).Once()
			},
			checkMock: func(t *testing.T, m *MockSecretSaver, req Request) {
				// Optional: More detailed check if needed, but MatchedBy covers format
			},
			expectedStatus: http.StatusOK,
			// Expected body structure, Alias and Key will be checked dynamically
			expectedBody: Response{
				Response: resp.OK(),
				Alias:    "", // Placeholder
				Key:      "", // Placeholder
			},
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder, expectedBody interface{}) {
				var respBody Response
				err := json.Unmarshal(rr.Body.Bytes(), &respBody)
				require.NoError(t, err)

				assert.Equal(t, "OK", respBody.Status)
				assert.True(t, uuidRegex.MatchString(respBody.Alias), "Alias should be a valid UUID")
				assert.True(t, keyRegex.MatchString(respBody.Key), "Key should be a valid hex key")
				assert.Len(t, respBody.Key, 32, "Key should be 32 hex characters (16 bytes)") // Assuming GenerateRandomHexKey returns 16 bytes
			},
		},
		{
			name: "Success Save One Time",
			requestBody: newJsonRequest(t, Request{
				Message:    "one time secret",
				Expiration: 1, // 1 hour
				OneTime:    true,
			}),
			setupMock: func(m *MockSecretSaver) {
				m.On("Set",
					mock.MatchedBy(func(key string) bool { return uuidRegex.MatchString(key) }),
					mock.AnythingOfType("[]uint8"),
					time.Duration(1)*time.Hour,
				).Return(nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody: Response{
				Response: resp.OK(),
				Alias:    "",
				Key:      "",
			},
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder, expectedBody interface{}) {
				var respBody Response
				err := json.Unmarshal(rr.Body.Bytes(), &respBody)
				require.NoError(t, err)
				assert.Equal(t, "OK", respBody.Status)
				assert.True(t, uuidRegex.MatchString(respBody.Alias), "Alias should be a valid UUID")
				assert.True(t, keyRegex.MatchString(respBody.Key), "Key should be a valid hex key")
			},
		},
		{
			name: "Success Save Zero Expiration (No TTL)",
			requestBody: newJsonRequest(t, Request{
				Message:    "no expiration",
				Expiration: 0, // Should result in 0 TTL
				OneTime:    false,
			}),
			setupMock: func(m *MockSecretSaver) {
				m.On("Set",
					mock.MatchedBy(func(key string) bool { return uuidRegex.MatchString(key) }),
					mock.AnythingOfType("[]uint8"),
					time.Duration(0), // Expect 0 TTL
				).Return(nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody: Response{
				Response: resp.OK(),
				Alias:    "",
				Key:      "",
			},
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder, expectedBody interface{}) {
				var respBody Response
				err := json.Unmarshal(rr.Body.Bytes(), &respBody)
				require.NoError(t, err)
				assert.Equal(t, "OK", respBody.Status)
				assert.True(t, uuidRegex.MatchString(respBody.Alias), "Alias should be a valid UUID")
				assert.True(t, keyRegex.MatchString(respBody.Key), "Key should be a valid hex key")
			},
		},
		{
			name:        "Error Invalid JSON Syntax",
			requestBody: bytes.NewBufferString(`{"message": "hello", "expiration": 1,`), // Malformed JSON
			setupMock: func(m *MockSecretSaver) {
				// Set should not be called
			},
			expectedStatus: http.StatusBadRequest,
			// Check the specific error message format from the handler
			expectedBody: resp.Error("Failed to read or decode request body."),
		},
		{
			name: "Error JSON Type Mismatch",
			requestBody: newJsonRequest(t, map[string]interface{}{ // Use map for type mismatch
				"message":    "hello",
				"expiration": "not-an-int", // Wrong type
				"one_time":   false,
			}),
			setupMock: func(m *MockSecretSaver) {
				// Set should not be called
			},
			expectedStatus: http.StatusBadRequest,
			// Check the specific error message format from the handler
			expectedBody: resp.Error("Invalid type for field 'expiration'. Expected type 'int' but received JSON string."),
		},
		{
			name: "Error Validation Failed (Missing Message)",
			requestBody: newJsonRequest(t, Request{
				Message:    "", // Missing required field
				Expiration: 1,
				OneTime:    false,
			}),
			setupMock: func(m *MockSecretSaver) {
				// Set should not be called
			},
			expectedStatus: http.StatusBadRequest,
			// Use the specific validation error response structure
			expectedBody: resp.ValidationErrorResponse([]resp.ValidationError{
				{Field: "message", Error: "This field is required"},
			}),
		},
		{
			name: "Error Secret Saver Fails",
			requestBody: newJsonRequest(t, Request{
				Message:    "save should fail",
				Expiration: 5,
				OneTime:    false,
			}),
			setupMock: func(m *MockSecretSaver) {
				// Mock Set to return an error
				m.On("Set",
					mock.MatchedBy(func(key string) bool { return uuidRegex.MatchString(key) }),
					mock.AnythingOfType("[]uint8"),
					time.Duration(5)*time.Hour,
				).Return(errors.New("redis connection error")).Once() // Simulate storage error
			},
			expectedStatus: http.StatusInternalServerError,
			// Check the specific error message returned by the handler in this case
			expectedBody: resp.Error("Url already exists"), // The handler currently returns this specific message on ANY Set error
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockSaver := new(MockSecretSaver)
			if tc.setupMock != nil {
				tc.setupMock(mockSaver)
			}

			handler := New(log, mockSaver)

			req := httptest.NewRequest(http.MethodPost, "/save", tc.requestBody)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Assert Status Code
			assert.Equal(t, tc.expectedStatus, rr.Code, "Status code mismatch")

			// Assert Body
			if tc.checkResponse != nil {
				// Use custom check for dynamic fields (like alias, key)
				tc.checkResponse(t, rr, tc.expectedBody)
			} else {
				// Use standard JSON comparison for static bodies
				expectedJson, err := json.Marshal(tc.expectedBody)
				require.NoError(t, err)
				assert.JSONEq(t, string(expectedJson), rr.Body.String(), "Response body mismatch")
			}

			// Assert Mock Calls
			mockSaver.AssertExpectations(t) // Verify all expected calls were made
		})
	}
}

package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"yoopass-api/internal/dto"
	resp "yoopass-api/internal/http-server/handlers/response"
	cipher "yoopass-api/internal/tools/cipher" // Assuming cipher package exists and works

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSecretFetcher is a mock type for the SecretFetcher interface
type MockSecretFetcher struct {
	mock.Mock
}

func (m *MockSecretFetcher) Fetch(key string) ([]byte, error) {
	args := m.Called(key)
	// Handle nil byte slice correctly
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSecretFetcher) Delete(key string) error {
	args := m.Called(key)
	return args.Error(0)
}

// Helper to create a chi context with URL parameters
func chiCtx(alias, key string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("alias", alias)
	rctx.URLParams.Add("key", key)
	return context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
}

// Helper to encode data for tests (replace with actual cipher logic if needed)
func encodeForTest(t *testing.T, data dto.Secret, key string) []byte {
	t.Helper()
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)
	encodedData, err := cipher.Encode(jsonData, key) // Use the actual Encode function
	require.NoError(t, err)
	return encodedData
}

func TestFetchHandler(t *testing.T) {
	// Discard logger for cleaner test output
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})).With(slog.String("test", "fetch"))
	// Use slog.New(slog.NewJSONHandler(io.Discard, nil)) if you don't want any logs during tests

	testCases := []struct {
		name           string
		alias          string
		key            string
		setupMock      func(m *MockSecretFetcher, alias, key string)
		expectedStatus int
		expectedBody   interface{} // Can be Response or resp.Response
		checkMockCalls func(t *testing.T, m *MockSecretFetcher, alias string)
	}{
		{
			name:  "Success Fetch Regular Secret",
			alias: "f7ab603e-fbae-4182-8379-8763d9327d51",
			key:   "46da5d3577209271242b42882a034c3d",
			setupMock: func(m *MockSecretFetcher, alias, key string) {
				secretData := dto.Secret{Message: "hello world", OneTime: false}
				encodedData := encodeForTest(t, secretData, key)
				m.On("Fetch", alias).Return(encodedData, nil).Once()
				// Delete should NOT be called
			},
			expectedStatus: http.StatusOK,
			expectedBody: Response{
				Response: resp.OK(),
				Message:  "hello world",
			},
			checkMockCalls: func(t *testing.T, m *MockSecretFetcher, alias string) {
				m.AssertCalled(t, "Fetch", alias)
				m.AssertNotCalled(t, "Delete", alias)
			},
		},
		{
			name:  "Success Fetch One-Time Secret",
			alias: "f7ab603e-fbae-4182-8379-8763d9327d22",
			key:   "46da5d3577209271242b42882a034c3d",
			setupMock: func(m *MockSecretFetcher, alias, key string) {
				secretData := dto.Secret{Message: "this will vanish", OneTime: true}
				encodedData := encodeForTest(t, secretData, key)
				m.On("Fetch", alias).Return(encodedData, nil).Once()
				m.On("Delete", alias).Return(nil).Once() // Expect Delete to be called
			},
			expectedStatus: http.StatusOK,
			expectedBody: Response{
				Response: resp.OK(),
				Message:  "this will vanish",
			},
			checkMockCalls: func(t *testing.T, m *MockSecretFetcher, alias string) {
				m.AssertCalled(t, "Fetch", alias)
				m.AssertCalled(t, "Delete", alias)
			},
		},
		{
			name:  "Error Fetch One-Time Secret Delete Fails",
			alias: "f7ab603e-fbae-4182-8379-8763d9327d51",
			key:   "46da5d3577209271242b42882a034c3d",
			setupMock: func(m *MockSecretFetcher, alias, key string) {
				secretData := dto.Secret{Message: "this should vanish but delete fails", OneTime: true}
				encodedData := encodeForTest(t, secretData, key)
				m.On("Fetch", alias).Return(encodedData, nil).Once()
				m.On("Delete", alias).Return(errors.New("db error")).Once() // Simulate delete failure
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   resp.Error("Failed to delete secret"),
			checkMockCalls: func(t *testing.T, m *MockSecretFetcher, alias string) {
				m.AssertCalled(t, "Fetch", alias)
				m.AssertCalled(t, "Delete", alias)
			},
		},
		{
			name:  "Error Missing Alias",
			alias: "", // Missing alias
			key:   "46da5d3577209271242b42882a034c3d",
			setupMock: func(m *MockSecretFetcher, alias, key string) {
				// Fetch/Delete should not be called
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   resp.Error("Alias parameter is missing"),
			checkMockCalls: func(t *testing.T, m *MockSecretFetcher, alias string) {
				m.AssertNotCalled(t, "Fetch", mock.Anything)
				m.AssertNotCalled(t, "Delete", mock.Anything)
			},
		},
		{
			name:  "Error Missing Key",
			alias: "f7ab603e-fbae-4182-8379-8763d9327d51",
			key:   "", // Missing key
			setupMock: func(m *MockSecretFetcher, alias, key string) {
				// Fetch/Delete should not be called
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   resp.Error("Key parameter is missing"),
			checkMockCalls: func(t *testing.T, m *MockSecretFetcher, alias string) {
				m.AssertNotCalled(t, "Fetch", mock.Anything)
				m.AssertNotCalled(t, "Delete", mock.Anything)
			},
		},
		{
			name:  "Error Secret Not Found",
			alias: "f7ab603e-fbae-4182-8379-8763d9327d52",
			key:   "46da5d3577209271242b42882a034c3d",
			setupMock: func(m *MockSecretFetcher, alias, key string) {
				m.On("Fetch", alias).Return(nil, nil).Once() // Simulate not found
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   resp.Error("Secret not found"),
			checkMockCalls: func(t *testing.T, m *MockSecretFetcher, alias string) {
				m.AssertCalled(t, "Fetch", alias)
				m.AssertNotCalled(t, "Delete", alias)
			},
		},
		{
			name:  "Error Fetch Failed",
			alias: "f7ab603e-fbae-4182-8379-8763d9327d52",
			key:   "46da5d3577209271242b42882a034c3d",
			setupMock: func(m *MockSecretFetcher, alias, key string) {
				m.On("Fetch", alias).Return(nil, errors.New("internal storage error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   resp.Error("internal storage error"),
			checkMockCalls: func(t *testing.T, m *MockSecretFetcher, alias string) {
				m.AssertCalled(t, "Fetch", alias)
				m.AssertNotCalled(t, "Delete", alias)
			},
		},
		{
			name:  "Error Unmarshal Failed (Bad Data)",
			alias: "f7ab603e-fbae-4182-8379-8763d9327d5x",
			key:   "46da5d3577209271242b42882a034c3d",
			setupMock: func(m *MockSecretFetcher, alias, key string) {
				// Encode some invalid JSON data
				invalidJsonData := []byte(`{"message": "hello", "onetime": true`) // Missing closing brace
				encodedData, err := cipher.Encode(invalidJsonData, key)
				require.NoError(t, err)
				m.On("Fetch", alias).Return(encodedData, nil).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   resp.Error("Secret unmarshalling failed"),
			checkMockCalls: func(t *testing.T, m *MockSecretFetcher, alias string) {
				m.AssertCalled(t, "Fetch", alias)
				m.AssertNotCalled(t, "Delete", alias)
			},
		},
		{
			name:  "Error Decode Failed (Wrong Key)",
			alias: "f7ab603e-fbae-4182-8379-8763d9327d51",
			key:   "46da5d3577209271242b42882a034c3e", // Use a different key than encoding
			setupMock: func(m *MockSecretFetcher, alias, key string) {
				secretData := dto.Secret{Message: "cant decode this", OneTime: false}
				// Encode with the *correct* key for storage
				encodedData := encodeForTest(t, secretData, "46da5d3577209271242b42882a034c3d")
				m.On("Fetch", alias).Return(encodedData, nil).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			// The actual error message comes from the cipher package, check your implementation
			// For this test, we check the handler's generic error message.
			expectedBody: resp.Error("Failed to decode secret"), // Note: Handler logs "Failed to encode", should be "decode"
			checkMockCalls: func(t *testing.T, m *MockSecretFetcher, alias string) {
				m.AssertCalled(t, "Fetch", alias)
				m.AssertNotCalled(t, "Delete", alias)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockFetcher := new(MockSecretFetcher)
			if tc.setupMock != nil {
				tc.setupMock(mockFetcher, tc.alias, tc.key)
			}

			handler := New(log, mockFetcher)

			req := httptest.NewRequest(http.MethodGet, "/fetch/{alias}/{key}", nil)
			// Add chi context with URL parameters
			req = req.WithContext(chiCtx(tc.alias, tc.key))

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Assert Status Code
			assert.Equal(t, tc.expectedStatus, rr.Code)

			// Assert Body
			expectedJson, err := json.Marshal(tc.expectedBody)
			require.NoError(t, err)
			assert.JSONEq(t, string(expectedJson), rr.Body.String())

			// Assert Mock Calls (if check defined)
			if tc.checkMockCalls != nil {
				tc.checkMockCalls(t, mockFetcher, tc.alias)
			}
			mockFetcher.AssertExpectations(t) // Verify all expected calls were made
		})
	}
}

// Optional: Test the chi context helper itself
func TestChiCtxHelper(t *testing.T) {
	aliasVal := "myalias"
	keyVal := "mykey"
	ctx := chiCtx(aliasVal, keyVal)

	assert.Equal(t, aliasVal, chi.URLParamFromCtx(ctx, "alias"))
	assert.Equal(t, keyVal, chi.URLParamFromCtx(ctx, "key"))
	assert.Empty(t, chi.URLParamFromCtx(ctx, "nonexistent"))
}

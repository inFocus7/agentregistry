package v0_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/agentregistry-dev/agentregistry/internal/cli/agent/frameworks/common"
	agentmodels "github.com/agentregistry-dev/agentregistry/internal/models"
	v0 "github.com/agentregistry-dev/agentregistry/internal/registry/api/handlers/v0"
	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	"github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/internal/registry/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAgentsEndpoint(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	// Setup test data
	_, err := registryService.CreateAgent(ctx, &agentmodels.AgentJSON{
		AgentManifest: common.AgentManifest{
			Name:        "com.example.agent-alpha",
			Description: "Alpha test agent",
			Language:    "python",
			Framework:   "adk",
		},
		Version: "1.0.0",
	})
	require.NoError(t, err)
	// Approve and publish the agents so they're visible via public endpoints
	err = registryService.ApproveAgent(ctx, "com.example.agent-alpha", "1.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishAgent(ctx, "com.example.agent-alpha", "1.0.0")
	require.NoError(t, err)

	_, err = registryService.CreateAgent(ctx, &agentmodels.AgentJSON{
		AgentManifest: common.AgentManifest{
			Name:        "com.example.agent-beta",
			Description: "Beta test agent",
			Language:    "python",
			Framework:   "adk",
		},
		Version: "2.0.0",
	})
	require.NoError(t, err)
	// Approve and publish the agents so they're visible via public endpoints
	err = registryService.ApproveAgent(ctx, "com.example.agent-beta", "2.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishAgent(ctx, "com.example.agent-beta", "2.0.0")
	require.NoError(t, err)

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterAgentsEndpoints(api, "/v0", registryService, false)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		expectedCount  int
		expectedError  string
	}{
		{
			name:           "list all agents",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "list with limit",
			queryParams:    "?limit=1",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "search agents",
			queryParams:    "?search=alpha",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "filter latest only",
			queryParams:    "?version=latest",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "invalid limit",
			queryParams:    "?limit=abc",
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v0/agents"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp agentmodels.AgentListResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Len(t, resp.Agents, tt.expectedCount)
				assert.Equal(t, tt.expectedCount, resp.Metadata.Count)

				// Verify structure
				for _, agent := range resp.Agents {
					assert.NotEmpty(t, agent.Agent.Name)
					assert.NotEmpty(t, agent.Agent.Description)
					assert.NotNil(t, agent.Meta.Official)
				}
			} else if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestGetLatestAgentVersionEndpoint(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	// Setup test data
	_, err := registryService.CreateAgent(ctx, &agentmodels.AgentJSON{
		AgentManifest: common.AgentManifest{
			Name:        "com.example.detail-agent",
			Description: "Agent for detail testing",
			Language:    "python",
			Framework:   "adk",
		},
		Version: "1.0.0",
	})
	require.NoError(t, err)
	// Approve and publish the agent so it's visible via public endpoints
	err = registryService.ApproveAgent(ctx, "com.example.detail-agent", "1.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishAgent(ctx, "com.example.detail-agent", "1.0.0")
	require.NoError(t, err)

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterAgentsEndpoints(api, "/v0", registryService, false)

	tests := []struct {
		name           string
		agentName      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "get existing agent latest version",
			agentName:      "com.example.detail-agent",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "get non-existent agent",
			agentName:      "com.example.non-existent",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Agent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// URL encode the agent name
			encodedName := url.PathEscape(tt.agentName)
			req := httptest.NewRequest(http.MethodGet, "/v0/agents/"+encodedName+"/versions/latest", nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp agentmodels.AgentResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Equal(t, tt.agentName, resp.Agent.Name)
				assert.NotNil(t, resp.Meta.Official)
			} else if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestGetAgentVersionEndpoint(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	agentName := "com.example.version-agent"

	// Setup test data with multiple versions
	_, err := registryService.CreateAgent(ctx, &agentmodels.AgentJSON{
		AgentManifest: common.AgentManifest{
			Name:        agentName,
			Description: "Version test agent v1",
			Language:    "python",
			Framework:   "adk",
		},
		Version: "1.0.0",
	})
	require.NoError(t, err)
	// Approve and publish the agent so it's visible via public endpoints
	err = registryService.ApproveAgent(ctx, agentName, "1.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishAgent(ctx, agentName, "1.0.0")
	require.NoError(t, err)

	_, err = registryService.CreateAgent(ctx, &agentmodels.AgentJSON{
		AgentManifest: common.AgentManifest{
			Name:        agentName,
			Description: "Version test agent v2",
			Language:    "python",
			Framework:   "adk",
		},
		Version: "2.0.0",
	})
	require.NoError(t, err)
	err = registryService.ApproveAgent(ctx, agentName, "2.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishAgent(ctx, agentName, "2.0.0")
	require.NoError(t, err)

	// Add version with build metadata for URL encoding test
	_, err = registryService.CreateAgent(ctx, &agentmodels.AgentJSON{
		AgentManifest: common.AgentManifest{
			Name:        agentName,
			Description: "Version test agent with build metadata",
			Language:    "python",
			Framework:   "adk",
		},
		Version: "1.0.0+20130313144700",
	})
	require.NoError(t, err)
	err = registryService.ApproveAgent(ctx, agentName, "1.0.0+20130313144700", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishAgent(ctx, agentName, "1.0.0+20130313144700")
	require.NoError(t, err)

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterAgentsEndpoints(api, "/v0", registryService, false)

	tests := []struct {
		name           string
		agentName      string
		version        string
		expectedStatus int
		expectedError  string
		checkResult    func(*testing.T, *agentmodels.AgentResponse)
	}{
		{
			name:           "get existing version",
			agentName:      agentName,
			version:        "1.0.0",
			expectedStatus: http.StatusOK,
			checkResult: func(t *testing.T, resp *agentmodels.AgentResponse) {
				t.Helper()
				assert.Equal(t, "1.0.0", resp.Agent.Version)
				assert.Equal(t, "Version test agent v1", resp.Agent.Description)
				assert.False(t, resp.Meta.Official.IsLatest)
			},
		},
		{
			name:           "get latest version",
			agentName:      agentName,
			version:        "2.0.0",
			expectedStatus: http.StatusOK,
			checkResult: func(t *testing.T, resp *agentmodels.AgentResponse) {
				t.Helper()
				assert.Equal(t, "2.0.0", resp.Agent.Version)
				assert.True(t, resp.Meta.Official.IsLatest)
			},
		},
		{
			name:           "get non-existent version",
			agentName:      agentName,
			version:        "3.0.0",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Agent not found",
		},
		{
			name:           "get non-existent agent",
			agentName:      "com.example.non-existent",
			version:        "1.0.0",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Agent not found",
		},
		{
			name:           "get version with build metadata (URL encoded)",
			agentName:      agentName,
			version:        "1.0.0+20130313144700",
			expectedStatus: http.StatusOK,
			checkResult: func(t *testing.T, resp *agentmodels.AgentResponse) {
				t.Helper()
				assert.Equal(t, "1.0.0+20130313144700", resp.Agent.Version)
				assert.Equal(t, "Version test agent with build metadata", resp.Agent.Description)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// URL encode the agent name and version
			encodedName := url.PathEscape(tt.agentName)
			encodedVersion := url.PathEscape(tt.version)
			req := httptest.NewRequest(http.MethodGet, "/v0/agents/"+encodedName+"/versions/"+encodedVersion, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp agentmodels.AgentResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Equal(t, tt.agentName, resp.Agent.Name)
				assert.Equal(t, tt.version, resp.Agent.Version)
				assert.NotNil(t, resp.Meta.Official)

				if tt.checkResult != nil {
					tt.checkResult(t, &resp)
				}
			} else if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestGetAllAgentVersionsEndpoint(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	agentName := "com.example.multi-version-agent"

	// Setup test data with multiple versions
	versions := []string{"1.0.0", "1.1.0", "2.0.0"}
	for _, version := range versions {
		_, err := registryService.CreateAgent(ctx, &agentmodels.AgentJSON{
			AgentManifest: common.AgentManifest{
				Name:        agentName,
				Description: "Multi-version test agent " + version,
				Language:    "python",
				Framework:   "adk",
			},
			Version: version,
		})
		require.NoError(t, err)
		// Approve and publish each version so it's visible via public endpoints
		err = registryService.ApproveAgent(ctx, agentName, version, "Test approval")
		require.NoError(t, err)
		err = registryService.PublishAgent(ctx, agentName, version)
		require.NoError(t, err)
	}

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterAgentsEndpoints(api, "/v0", registryService, false)

	tests := []struct {
		name           string
		agentName      string
		expectedStatus int
		expectedCount  int
		expectedError  string
	}{
		{
			name:           "get all versions of existing agent",
			agentName:      agentName,
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "get versions of non-existent agent",
			agentName:      "com.example.non-existent",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Agent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// URL encode the agent name
			encodedName := url.PathEscape(tt.agentName)
			req := httptest.NewRequest(http.MethodGet, "/v0/agents/"+encodedName+"/versions", nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp agentmodels.AgentListResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Len(t, resp.Agents, tt.expectedCount)
				assert.Equal(t, tt.expectedCount, resp.Metadata.Count)

				// Verify all versions are for the same agent
				for _, agent := range resp.Agents {
					assert.Equal(t, tt.agentName, agent.Agent.Name)
					assert.NotNil(t, agent.Meta.Official)
				}

				// Verify all expected versions are present
				versionSet := make(map[string]bool)
				for _, agent := range resp.Agents {
					versionSet[agent.Agent.Version] = true
				}
				for _, expectedVersion := range versions {
					assert.True(t, versionSet[expectedVersion], "Version %s should be present", expectedVersion)
				}

				// Verify exactly one is marked as latest
				latestCount := 0
				for _, agent := range resp.Agents {
					if agent.Meta.Official.IsLatest {
						latestCount++
					}
				}
				assert.Equal(t, 1, latestCount, "Exactly one version should be marked as latest")
			} else if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestAgentsEndpointEdgeCases(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	// Setup test data with edge case names that comply with constraints
	specialAgents := []struct {
		name        string
		description string
		version     string
	}{
		{"io.dots.and-dashes.agent-name", "Agent with dots and dashes", "1.0.0"},
		{"com.long-namespace-name.very-long-agent-name-here", "Long names", "1.0.0"},
		{"org.test123.agent-with-dashes", "Agent with dashes", "1.0.0"},
	}

	for _, agent := range specialAgents {
		_, err := registryService.CreateAgent(ctx, &agentmodels.AgentJSON{
			AgentManifest: common.AgentManifest{
				Name:        agent.name,
				Description: agent.description,
				Language:    "python",
				Framework:   "adk",
			},
			Version: agent.version,
		})
		require.NoError(t, err)
		// Approve and publish each agent so it's visible via public endpoints
		err = registryService.ApproveAgent(ctx, agent.name, agent.version, "Test approval")
		require.NoError(t, err)
		err = registryService.PublishAgent(ctx, agent.name, agent.version)
		require.NoError(t, err)
	}

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterAgentsEndpoints(api, "/v0", registryService, false)

	t.Run("URL encoding edge cases", func(t *testing.T) {
		tests := []struct {
			name      string
			agentName string
		}{
			{"dots and dashes", "io.dots.and-dashes.agent-name"},
			{"long agent name", "com.long-namespace-name.very-long-agent-name-here"},
			{"dashes", "org.test123.agent-with-dashes"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Test latest version endpoint
				encodedName := url.PathEscape(tt.agentName)
				req := httptest.NewRequest(http.MethodGet, "/v0/agents/"+encodedName+"/versions/latest", nil)
				w := httptest.NewRecorder()

				mux.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)

				var resp agentmodels.AgentResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Equal(t, tt.agentName, resp.Agent.Name)
			})
		}
	})

	t.Run("query parameter edge cases", func(t *testing.T) {
		tests := []struct {
			name           string
			queryParams    string
			expectedStatus int
			expectedError  string
		}{
			{"limit too high", "?limit=1000", http.StatusUnprocessableEntity, "validation failed"},
			{"negative limit", "?limit=-1", http.StatusUnprocessableEntity, "validation failed"},
			{"invalid updated_since format", "?updated_since=invalid", http.StatusBadRequest, "Invalid updated_since format"},
			{"future updated_since", "?updated_since=2030-01-01T00:00:00Z", http.StatusOK, ""},
			{"very old updated_since", "?updated_since=1990-01-01T00:00:00Z", http.StatusOK, ""},
			{"empty search parameter", "?search=", http.StatusOK, ""},
			{"search with special characters", "?search=测试", http.StatusOK, ""},
			{"combined valid parameters", "?search=agent&limit=5&version=latest", http.StatusOK, ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/v0/agents"+tt.queryParams, nil)
				w := httptest.NewRecorder()

				mux.ServeHTTP(w, req)

				assert.Equal(t, tt.expectedStatus, w.Code)

				if tt.expectedStatus == http.StatusOK {
					var resp agentmodels.AgentListResponse
					err := json.NewDecoder(w.Body).Decode(&resp)
					assert.NoError(t, err)
					assert.NotNil(t, resp.Metadata)
				} else if tt.expectedError != "" {
					assert.Contains(t, w.Body.String(), tt.expectedError)
				}
			})
		}
	})

	t.Run("response structure validation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v0/agents", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp agentmodels.AgentListResponse
		err := json.NewDecoder(w.Body).Decode(&resp)
		assert.NoError(t, err)

		// Verify metadata structure
		assert.NotNil(t, resp.Metadata)
		assert.GreaterOrEqual(t, resp.Metadata.Count, 0)

		// Verify each agent has complete structure
		for _, agent := range resp.Agents {
			assert.NotEmpty(t, agent.Agent.Name)
			assert.NotEmpty(t, agent.Agent.Description)
			assert.NotEmpty(t, agent.Agent.Version)
			assert.NotNil(t, agent.Meta)
			assert.NotNil(t, agent.Meta.Official)
			assert.NotZero(t, agent.Meta.Official.PublishedAt)
		}
	})
}

func TestDeleteAgentVersionEndpoint(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	agentName := "com.example.delete-agent"
	version := "1.0.0"

	// Setup test data
	_, err := registryService.CreateAgent(ctx, &agentmodels.AgentJSON{
		AgentManifest: common.AgentManifest{
			Name:        agentName,
			Description: "Agent for deletion testing",
			Language:    "python",
			Framework:   "adk",
		},
		Version: version,
	})
	require.NoError(t, err)

	// Create API with admin mode enabled
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterAgentsEndpoints(api, "/v0", registryService, true)

	t.Run("delete existing agent version", func(t *testing.T) {
		encodedName := url.PathEscape(agentName)
		encodedVersion := url.PathEscape(version)
		req := httptest.NewRequest(http.MethodDelete, "/v0/agents/"+encodedName+"/versions/"+encodedVersion, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp v0.EmptyResponse
		err := json.NewDecoder(w.Body).Decode(&resp)
		assert.NoError(t, err)
		assert.Contains(t, resp.Message, "deleted successfully")

		// Verify agent is actually deleted
		_, err = registryService.GetAgentByNameAndVersion(ctx, agentName, version)
		assert.Error(t, err)
	})

	t.Run("delete non-existent agent", func(t *testing.T) {
		encodedName := url.PathEscape("com.example.non-existent")
		encodedVersion := url.PathEscape("1.0.0")
		req := httptest.NewRequest(http.MethodDelete, "/v0/agents/"+encodedName+"/versions/"+encodedVersion, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Agent not found")
	})
}

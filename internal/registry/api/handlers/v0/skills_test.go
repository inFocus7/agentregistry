package v0_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	skillmodels "github.com/agentregistry-dev/agentregistry/internal/models"
	v0 "github.com/agentregistry-dev/agentregistry/internal/registry/api/handlers/v0"
	"github.com/agentregistry-dev/agentregistry/internal/registry/config"
	"github.com/agentregistry-dev/agentregistry/internal/registry/database"
	"github.com/agentregistry-dev/agentregistry/internal/registry/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListSkillsEndpoint(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	// Setup test data
	_, err := registryService.CreateSkill(ctx, &skillmodels.SkillJSON{
		Name:        "skill-alpha",
		Description: "Alpha test skill",
		Version:     "1.0.0",
	})
	require.NoError(t, err)
	// Approve and publish the skills so they're visible via public endpoints
	err = registryService.ApproveSkill(ctx, "skill-alpha", "1.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishSkill(ctx, "skill-alpha", "1.0.0")
	require.NoError(t, err)

	_, err = registryService.CreateSkill(ctx, &skillmodels.SkillJSON{
		Name:        "skill-beta",
		Description: "Beta test skill",
		Version:     "2.0.0",
	})
	require.NoError(t, err)
	// Approve and publish the skills so they're visible via public endpoints
	err = registryService.ApproveSkill(ctx, "skill-beta", "2.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishSkill(ctx, "skill-beta", "2.0.0")
	require.NoError(t, err)

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterSkillsEndpoints(api, "/v0", registryService, false)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		expectedCount  int
		expectedError  string
	}{
		{
			name:           "list all skills",
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
			name:           "search skills",
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
			req := httptest.NewRequest(http.MethodGet, "/v0/skills"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp skillmodels.SkillListResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Len(t, resp.Skills, tt.expectedCount)
				assert.Equal(t, tt.expectedCount, resp.Metadata.Count)

				// Verify structure
				for _, skill := range resp.Skills {
					assert.NotEmpty(t, skill.Skill.Name)
					assert.NotEmpty(t, skill.Skill.Description)
					assert.NotNil(t, skill.Meta.Official)
				}
			} else if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestGetLatestSkillVersionEndpoint(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	// Setup test data
	_, err := registryService.CreateSkill(ctx, &skillmodels.SkillJSON{
		Name:        "detail-skill",
		Description: "Skill for detail testing",
		Version:     "1.0.0",
	})
	require.NoError(t, err)
	// Approve and publish the skill so it's visible via public endpoints
	err = registryService.ApproveSkill(ctx, "detail-skill", "1.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishSkill(ctx, "detail-skill", "1.0.0")
	require.NoError(t, err)

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterSkillsEndpoints(api, "/v0", registryService, false)

	tests := []struct {
		name           string
		skillName      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "get existing skill latest version",
			skillName:      "detail-skill",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "get non-existent skill",
			skillName:      "non-existent",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Skill not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// URL encode the skill name
			encodedName := url.PathEscape(tt.skillName)
			req := httptest.NewRequest(http.MethodGet, "/v0/skills/"+encodedName+"/versions/latest", nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp skillmodels.SkillResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Equal(t, tt.skillName, resp.Skill.Name)
				assert.NotNil(t, resp.Meta.Official)
			} else if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestGetSkillVersionEndpoint(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	skillName := "version-skill"

	// Setup test data with multiple versions
	_, err := registryService.CreateSkill(ctx, &skillmodels.SkillJSON{
		Name:        skillName,
		Description: "Version test skill v1",
		Version:     "1.0.0",
	})
	require.NoError(t, err)
	// Approve and publish the skill so it's visible via public endpoints
	err = registryService.ApproveSkill(ctx, skillName, "1.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishSkill(ctx, skillName, "1.0.0")
	require.NoError(t, err)

	_, err = registryService.CreateSkill(ctx, &skillmodels.SkillJSON{
		Name:        skillName,
		Description: "Version test skill v2",
		Version:     "2.0.0",
	})
	require.NoError(t, err)
	err = registryService.ApproveSkill(ctx, skillName, "2.0.0", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishSkill(ctx, skillName, "2.0.0")
	require.NoError(t, err)

	// Add version with build metadata for URL encoding test
	_, err = registryService.CreateSkill(ctx, &skillmodels.SkillJSON{
		Name:        skillName,
		Description: "Version test skill with build metadata",
		Version:     "1.0.0+20130313144700",
	})
	require.NoError(t, err)
	err = registryService.ApproveSkill(ctx, skillName, "1.0.0+20130313144700", "Test approval")
	require.NoError(t, err)
	err = registryService.PublishSkill(ctx, skillName, "1.0.0+20130313144700")
	require.NoError(t, err)

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterSkillsEndpoints(api, "/v0", registryService, false)

	tests := []struct {
		name           string
		skillName      string
		version        string
		expectedStatus int
		expectedError  string
		checkResult    func(*testing.T, *skillmodels.SkillResponse)
	}{
		{
			name:           "get existing version",
			skillName:      skillName,
			version:        "1.0.0",
			expectedStatus: http.StatusOK,
			checkResult: func(t *testing.T, resp *skillmodels.SkillResponse) {
				t.Helper()
				assert.Equal(t, "1.0.0", resp.Skill.Version)
				assert.Equal(t, "Version test skill v1", resp.Skill.Description)
				assert.False(t, resp.Meta.Official.IsLatest)
			},
		},
		{
			name:           "get latest version",
			skillName:      skillName,
			version:        "2.0.0",
			expectedStatus: http.StatusOK,
			checkResult: func(t *testing.T, resp *skillmodels.SkillResponse) {
				t.Helper()
				assert.Equal(t, "2.0.0", resp.Skill.Version)
				assert.True(t, resp.Meta.Official.IsLatest)
			},
		},
		{
			name:           "get non-existent version",
			skillName:      skillName,
			version:        "3.0.0",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Skill not found",
		},
		{
			name:           "get non-existent skill",
			skillName:      "non-existent",
			version:        "1.0.0",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Skill not found",
		},
		{
			name:           "get version with build metadata (URL encoded)",
			skillName:      skillName,
			version:        "1.0.0+20130313144700",
			expectedStatus: http.StatusOK,
			checkResult: func(t *testing.T, resp *skillmodels.SkillResponse) {
				t.Helper()
				assert.Equal(t, "1.0.0+20130313144700", resp.Skill.Version)
				assert.Equal(t, "Version test skill with build metadata", resp.Skill.Description)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// URL encode the skill name and version
			encodedName := url.PathEscape(tt.skillName)
			encodedVersion := url.PathEscape(tt.version)
			req := httptest.NewRequest(http.MethodGet, "/v0/skills/"+encodedName+"/versions/"+encodedVersion, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp skillmodels.SkillResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Equal(t, tt.skillName, resp.Skill.Name)
				assert.Equal(t, tt.version, resp.Skill.Version)
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

func TestGetAllSkillVersionsEndpoint(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	skillName := "multi-version-skill"

	// Setup test data with multiple versions
	versions := []string{"1.0.0", "1.1.0", "2.0.0"}
	for _, version := range versions {
		_, err := registryService.CreateSkill(ctx, &skillmodels.SkillJSON{
			Name:        skillName,
			Description: "Multi-version test skill " + version,
			Version:     version,
		})
		require.NoError(t, err)
		// Approve and publish each version so it's visible via public endpoints
		err = registryService.ApproveSkill(ctx, skillName, version, "Test approval")
		require.NoError(t, err)
		err = registryService.PublishSkill(ctx, skillName, version)
		require.NoError(t, err)
	}

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterSkillsEndpoints(api, "/v0", registryService, false)

	tests := []struct {
		name           string
		skillName      string
		expectedStatus int
		expectedCount  int
		expectedError  string
	}{
		{
			name:           "get all versions of existing skill",
			skillName:      skillName,
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "get versions of non-existent skill",
			skillName:      "non-existent",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Skill not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// URL encode the skill name
			encodedName := url.PathEscape(tt.skillName)
			req := httptest.NewRequest(http.MethodGet, "/v0/skills/"+encodedName+"/versions", nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp skillmodels.SkillListResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Len(t, resp.Skills, tt.expectedCount)
				assert.Equal(t, tt.expectedCount, resp.Metadata.Count)

				// Verify all versions are for the same skill
				for _, skill := range resp.Skills {
					assert.Equal(t, tt.skillName, skill.Skill.Name)
					assert.NotNil(t, skill.Meta.Official)
				}

				// Verify all expected versions are present
				versionSet := make(map[string]bool)
				for _, skill := range resp.Skills {
					versionSet[skill.Skill.Version] = true
				}
				for _, expectedVersion := range versions {
					assert.True(t, versionSet[expectedVersion], "Version %s should be present", expectedVersion)
				}

				// Verify exactly one is marked as latest
				latestCount := 0
				for _, skill := range resp.Skills {
					if skill.Meta.Official.IsLatest {
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

func TestSkillsEndpointEdgeCases(t *testing.T) {
	ctx := context.Background()
	registryService := service.NewRegistryService(database.NewTestDB(t), config.NewConfig())

	// Setup test data with edge case names that comply with constraints
	specialSkills := []struct {
		name        string
		description string
		version     string
	}{
		{"skill-with-dashes", "Skill with dashes", "1.0.0"},
		{"very-long-skill-name-here", "Long names", "1.0.0"},
		{"skill_with_underscores", "Skill with underscores", "1.0.0"},
	}

	for _, skill := range specialSkills {
		_, err := registryService.CreateSkill(ctx, &skillmodels.SkillJSON{
			Name:        skill.name,
			Description: skill.description,
			Version:     skill.version,
		})
		require.NoError(t, err)
		// Approve and publish each skill so it's visible via public endpoints
		err = registryService.ApproveSkill(ctx, skill.name, skill.version, "Test approval")
		require.NoError(t, err)
		err = registryService.PublishSkill(ctx, skill.name, skill.version)
		require.NoError(t, err)
	}

	// Create API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterSkillsEndpoints(api, "/v0", registryService, false)

	t.Run("URL encoding edge cases", func(t *testing.T) {
		tests := []struct {
			name      string
			skillName string
		}{
			{"dashes", "skill-with-dashes"},
			{"long skill name", "very-long-skill-name-here"},
			{"underscores", "skill_with_underscores"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Test latest version endpoint
				encodedName := url.PathEscape(tt.skillName)
				req := httptest.NewRequest(http.MethodGet, "/v0/skills/"+encodedName+"/versions/latest", nil)
				w := httptest.NewRecorder()

				mux.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)

				var resp skillmodels.SkillResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				assert.NoError(t, err)
				assert.Equal(t, tt.skillName, resp.Skill.Name)
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
			{"combined valid parameters", "?search=skill&limit=5&version=latest", http.StatusOK, ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/v0/skills"+tt.queryParams, nil)
				w := httptest.NewRecorder()

				mux.ServeHTTP(w, req)

				assert.Equal(t, tt.expectedStatus, w.Code)

				if tt.expectedStatus == http.StatusOK {
					var resp skillmodels.SkillListResponse
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
		req := httptest.NewRequest(http.MethodGet, "/v0/skills", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp skillmodels.SkillListResponse
		err := json.NewDecoder(w.Body).Decode(&resp)
		assert.NoError(t, err)

		// Verify metadata structure
		assert.NotNil(t, resp.Metadata)
		assert.GreaterOrEqual(t, resp.Metadata.Count, 0)

		// Verify each skill has complete structure
		for _, skill := range resp.Skills {
			assert.NotEmpty(t, skill.Skill.Name)
			assert.NotEmpty(t, skill.Skill.Description)
			assert.NotEmpty(t, skill.Skill.Version)
			assert.NotNil(t, skill.Meta)
			assert.NotNil(t, skill.Meta.Official)
			assert.NotZero(t, skill.Meta.Official.PublishedAt)
		}
	})
}

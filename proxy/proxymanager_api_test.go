package proxy

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mostlygeek/llama-swap/proxy/config"
	"github.com/stretchr/testify/require"
)

func TestProxyManager_ParseMetricsRangeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pm := &ProxyManager{
		config: config.Config{
			MetricsQueryMaxRows: 250,
		},
	}

	tests := []struct {
		name           string
		target         string
		wantRange      string
		wantDuration   time.Duration
		wantFrom       bool
		wantTo         bool
		wantLimit      int
		wantErr        string
		wantExactFrom  time.Time
		wantExactTo    time.Time
		wantFromToSame bool
	}{
		{
			name:         "past 5 minutes",
			target:       "/api/metrics?range=5m",
			wantRange:    "5m",
			wantDuration: 5 * time.Minute,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past 10 minutes",
			target:       "/api/metrics?range=10m",
			wantRange:    "10m",
			wantDuration: 10 * time.Minute,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past 1 hour",
			target:       "/api/metrics?range=1h",
			wantRange:    "1h",
			wantDuration: time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past 8 hours",
			target:       "/api/metrics?range=8h",
			wantRange:    "8h",
			wantDuration: 8 * time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past day",
			target:       "/api/metrics?range=1d",
			wantRange:    "1d",
			wantDuration: 24 * time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past week",
			target:       "/api/metrics?range=1w",
			wantRange:    "1w",
			wantDuration: 7 * 24 * time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past month",
			target:       "/api/metrics?range=1mo",
			wantRange:    "1mo",
			wantDuration: 30 * 24 * time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:      "all",
			target:    "/api/metrics?range=all",
			wantRange: "all",
			wantLimit: 250,
		},
		{
			name:          "custom with RFC3339 from and to",
			target:        "/api/metrics?range=custom&from=2026-04-20T10:00:00Z&to=2026-04-20T11:00:00Z",
			wantRange:     "custom",
			wantFrom:      true,
			wantTo:        true,
			wantLimit:     250,
			wantExactFrom: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
			wantExactTo:   time.Date(2026, 4, 20, 11, 0, 0, 0, time.UTC),
		},
		{
			name:          "custom with unix millisecond from",
			target:        "/api/metrics?range=custom&from=1776682800123",
			wantRange:     "custom",
			wantFrom:      true,
			wantLimit:     250,
			wantExactFrom: time.UnixMilli(1776682800123),
		},
		{
			name:      "limit clamps to configured max",
			target:    "/api/metrics?range=all&limit=999",
			wantRange: "all",
			wantLimit: 250,
		},
		{
			name:      "limit accepts smaller positive value",
			target:    "/api/metrics?range=all&limit=25",
			wantRange: "all",
			wantLimit: 25,
		},
		{
			name:    "custom requires a bound",
			target:  "/api/metrics?range=custom",
			wantErr: "custom range requires from or to",
		},
		{
			name:    "custom rejects reversed bounds",
			target:  "/api/metrics?range=custom&from=2026-04-20T11:00:00Z&to=2026-04-20T10:00:00Z",
			wantErr: "from must be before to",
		},
		{
			name:    "unsupported range",
			target:  "/api/metrics?range=2h",
			wantErr: `unsupported metrics range "2h"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", tt.target, nil)

			query, normalizedRange, err := pm.parseMetricsRangeQuery(c)
			after := time.Now()

			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantRange, normalizedRange)
			require.Equal(t, tt.wantLimit, query.Limit)

			if tt.wantFrom {
				require.NotNil(t, query.From)
			} else {
				require.Nil(t, query.From)
			}
			if tt.wantTo {
				require.NotNil(t, query.To)
			} else {
				require.Nil(t, query.To)
			}

			if tt.wantDuration > 0 {
				require.False(t, query.From.Before(before.Add(-tt.wantDuration)), "from should not be older than range duration")
				require.False(t, query.From.After(after.Add(-tt.wantDuration)), "from should not be newer than range duration")
			}
			if !tt.wantExactFrom.IsZero() {
				require.True(t, tt.wantExactFrom.Equal(*query.From), "unexpected exact from")
			}
			if !tt.wantExactTo.IsZero() {
				require.True(t, tt.wantExactTo.Equal(*query.To), "unexpected exact to")
			}
		})
	}
}

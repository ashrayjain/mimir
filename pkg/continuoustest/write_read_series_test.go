// SPDX-License-Identifier: AGPL-3.0-only

package continuoustest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type getMetricHistoryFunc func(test *WriteReadSeriesTest) *MetricHistory

type WriteReadSeriesTestTuple struct {
	metricName       string
	querySum         querySumFunc
	generateSeries   generateSeriesFunc
	generateValue    generateValueFunc
	getMetricHistory getMetricHistoryFunc
}

var (
	cfgSample        WriteReadSeriesTestConfig
	cfgHist          WriteReadSeriesTestConfig
	sampleTestTuples []WriteReadSeriesTestTuple
	histTestTuples   []WriteReadSeriesTestTuple
)

func init() {
	cfgSample = WriteReadSeriesTestConfig{}
	flagext.DefaultValues(&cfgSample)
	cfgSample.NumSeries = 2
	cfgSample.WithSamples = true

	cfgHist = WriteReadSeriesTestConfig{}
	flagext.DefaultValues(&cfgHist)
	cfgHist.NumSeries = 2
	cfgHist.WithHistograms = true

	sampleTestTuples = []WriteReadSeriesTestTuple{{
		metricName:     metricNameSample,
		querySum:       querySumSample,
		generateSeries: generateSineWaveSeries,
		generateValue:  generateSineWaveValue,
		getMetricHistory: func(test *WriteReadSeriesTest) *MetricHistory {
			return &test.sampleMetric
		},
	}}

	histTestTuples = make([]WriteReadSeriesTestTuple, 4)
	for i := range histTestTuples {
		i := i // shadowing it to ensure it's properly updated in the closure
		var generateValue generateValueFunc
		if i%2 == 0 {
			generateValue = generateHistogramIntValueAsFloat
		} else {
			generateValue = generateHistogramFloatValue
		}
		histTestTuples[i] = WriteReadSeriesTestTuple{
			metricName:     metricNamesHist[i],
			querySum:       querySumHist,
			generateSeries: generateHistogramSeries(i),
			generateValue:  generateValue,
			getMetricHistory: func(test *WriteReadSeriesTest) *MetricHistory {
				return &test.histMetrics[i]
			},
		}
	}
}

func TestWriteReadSeriesTest_Run(t *testing.T) {
	testWriteReadSeriesTest_Run(t, cfgSample, sampleTestTuples)
	testWriteReadSeriesTest_Run(t, cfgHist, histTestTuples)
}

func testWriteReadSeriesTest_Run(t *testing.T, cfg WriteReadSeriesTestConfig, testTuples []WriteReadSeriesTestTuple) {
	logger := log.NewNopLogger()
	multiplier := len(testTuples)

	t.Run("should write series with current timestamp if it's already aligned to write interval", func(t *testing.T) {
		client := &ClientMock{}
		client.On("WriteSeries", mock.Anything, mock.Anything).Return(200, nil)
		client.On("QueryRange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(model.Matrix{}, nil)
		client.On("Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(model.Vector{}, nil)

		reg := prometheus.NewPedanticRegistry()
		test := NewWriteReadSeriesTest(cfg, client, logger, reg)

		now := time.Unix(1000, 0)
		// Ignore this error. It will be non-nil because the query mock does not return any data.
		_ = test.Run(context.Background(), now)

		client.AssertNumberOfCalls(t, "WriteSeries", 1*multiplier)
		client.AssertNumberOfCalls(t, "QueryRange", 4*multiplier)
		client.AssertNumberOfCalls(t, "Query", 4*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)

			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, now, 2))
			assert.Equal(t, int64(1000), records.lastWrittenTimestamp.Unix())

			client.AssertCalled(t, "QueryRange", mock.Anything, tt.querySum(tt.metricName), time.Unix(1000, 0), time.Unix(1000, 0), writeInterval, mock.Anything)

			client.AssertCalled(t, "Query", mock.Anything, tt.querySum(tt.metricName), time.Unix(1000, 0), mock.Anything)
		}

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(fmt.Sprintf(`
			# HELP mimir_continuous_test_writes_total Total number of attempted write requests.
			# TYPE mimir_continuous_test_writes_total counter
			mimir_continuous_test_writes_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_total Total number of attempted query requests.
			# TYPE mimir_continuous_test_queries_total counter
			mimir_continuous_test_queries_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_failed_total Total number of failed query requests.
			# TYPE mimir_continuous_test_queries_failed_total counter
			mimir_continuous_test_queries_failed_total{test="write-read-series"} %d
		`, 1*multiplier, 8*multiplier, 0*multiplier)),
			"mimir_continuous_test_writes_total", "mimir_continuous_test_writes_failed_total",
			"mimir_continuous_test_queries_total", "mimir_continuous_test_queries_failed_total"))
	})

	t.Run("should write series with timestamp aligned to write interval", func(t *testing.T) {
		client := &ClientMock{}
		client.On("WriteSeries", mock.Anything, mock.Anything).Return(200, nil)
		client.On("QueryRange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(model.Matrix{}, nil)
		client.On("Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(model.Vector{}, nil)

		reg := prometheus.NewPedanticRegistry()
		test := NewWriteReadSeriesTest(cfg, client, logger, reg)

		now := time.Unix(999, 0)
		// Ignore this error. It will be non-nil because the query mock does not return any data.
		_ = test.Run(context.Background(), now)

		client.AssertNumberOfCalls(t, "WriteSeries", 1*multiplier)
		client.AssertNumberOfCalls(t, "QueryRange", 4*multiplier)
		client.AssertNumberOfCalls(t, "Query", 4*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)

			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, time.Unix(980, 0), 2))
			assert.Equal(t, int64(980), records.lastWrittenTimestamp.Unix())

			client.AssertCalled(t, "QueryRange", mock.Anything, tt.querySum(tt.metricName), time.Unix(980, 0), time.Unix(980, 0), writeInterval, mock.Anything)

			client.AssertCalled(t, "Query", mock.Anything, tt.querySum(tt.metricName), time.Unix(980, 0), mock.Anything)
		}

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(fmt.Sprintf(`
			# HELP mimir_continuous_test_writes_total Total number of attempted write requests.
			# TYPE mimir_continuous_test_writes_total counter
			mimir_continuous_test_writes_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_total Total number of attempted query requests.
			# TYPE mimir_continuous_test_queries_total counter
			mimir_continuous_test_queries_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_failed_total Total number of failed query requests.
			# TYPE mimir_continuous_test_queries_failed_total counter
			mimir_continuous_test_queries_failed_total{test="write-read-series"} %d
		`, 1*multiplier, 8*multiplier, 0*multiplier)),
			"mimir_continuous_test_writes_total", "mimir_continuous_test_writes_failed_total",
			"mimir_continuous_test_queries_total", "mimir_continuous_test_queries_failed_total"))
	})

	t.Run("should write series from last written timestamp until now", func(t *testing.T) {
		client := &ClientMock{}
		client.On("WriteSeries", mock.Anything, mock.Anything).Return(200, nil)
		client.On("QueryRange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(model.Matrix{}, nil)
		client.On("Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(model.Vector{}, nil)

		reg := prometheus.NewPedanticRegistry()
		test := NewWriteReadSeriesTest(cfg, client, logger, reg)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			records.lastWrittenTimestamp = time.Unix(940, 0)
		}

		now := time.Unix(1000, 0)
		// Ignore this error. It will be non-nil because the query mock does not return any data.
		_ = test.Run(context.Background(), now)

		client.AssertNumberOfCalls(t, "WriteSeries", 3*multiplier)
		client.AssertNumberOfCalls(t, "QueryRange", 4*multiplier)
		client.AssertNumberOfCalls(t, "Query", 4*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)

			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, time.Unix(960, 0), 2))
			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, time.Unix(980, 0), 2))
			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, time.Unix(1000, 0), 2))
			assert.Equal(t, int64(1000), records.lastWrittenTimestamp.Unix())

			client.AssertCalled(t, "QueryRange", mock.Anything, tt.querySum(tt.metricName), time.Unix(960, 0), time.Unix(1000, 0), writeInterval, mock.Anything)

			client.AssertCalled(t, "Query", mock.Anything, tt.querySum(tt.metricName), time.Unix(1000, 0), mock.Anything)
		}

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(fmt.Sprintf(`
			# HELP mimir_continuous_test_writes_total Total number of attempted write requests.
			# TYPE mimir_continuous_test_writes_total counter
			mimir_continuous_test_writes_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_total Total number of attempted query requests.
			# TYPE mimir_continuous_test_queries_total counter
			mimir_continuous_test_queries_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_failed_total Total number of failed query requests.
			# TYPE mimir_continuous_test_queries_failed_total counter
			mimir_continuous_test_queries_failed_total{test="write-read-series"} %d
		`, 3*multiplier, 8*multiplier, 0*multiplier)),
			"mimir_continuous_test_writes_total", "mimir_continuous_test_writes_failed_total",
			"mimir_continuous_test_queries_total", "mimir_continuous_test_queries_failed_total"))
	})

	t.Run("should stop remote writing on network error", func(t *testing.T) {
		client := &ClientMock{}
		client.On("WriteSeries", mock.Anything, mock.Anything).Return(0, errors.New("network error"))

		reg := prometheus.NewPedanticRegistry()
		test := NewWriteReadSeriesTest(cfg, client, logger, reg)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			records.lastWrittenTimestamp = time.Unix(940, 0)
		}
		now := time.Unix(1000, 0)
		err := test.Run(context.Background(), now)
		assert.Error(t, err)

		client.AssertNumberOfCalls(t, "WriteSeries", 1*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)

			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, time.Unix(960, 0), 2))
			assert.Equal(t, int64(940), records.lastWrittenTimestamp.Unix())
		}

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(fmt.Sprintf(`
			# HELP mimir_continuous_test_writes_total Total number of attempted write requests.
			# TYPE mimir_continuous_test_writes_total counter
			mimir_continuous_test_writes_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_writes_failed_total Total number of failed write requests.
			# TYPE mimir_continuous_test_writes_failed_total counter
			mimir_continuous_test_writes_failed_total{status_code="0",test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_total Total number of attempted query requests.
			# TYPE mimir_continuous_test_queries_total counter
			mimir_continuous_test_queries_total{test="write-read-series"} %d
		`, 1*multiplier, 1*multiplier, 0*multiplier)), "mimir_continuous_test_writes_total", "mimir_continuous_test_writes_failed_total", "mimir_continuous_test_queries_total"))
	})

	t.Run("should stop remote writing on 5xx error", func(t *testing.T) {
		client := &ClientMock{}
		client.On("WriteSeries", mock.Anything, mock.Anything).Return(500, errors.New("500 error"))

		reg := prometheus.NewPedanticRegistry()
		test := NewWriteReadSeriesTest(cfg, client, logger, reg)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			records.lastWrittenTimestamp = time.Unix(940, 0)
		}
		now := time.Unix(1000, 0)
		err := test.Run(context.Background(), now)
		assert.Error(t, err)

		client.AssertNumberOfCalls(t, "WriteSeries", 1*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)

			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, time.Unix(960, 0), 2))
			assert.Equal(t, int64(940), records.lastWrittenTimestamp.Unix())
		}

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(fmt.Sprintf(`
			# HELP mimir_continuous_test_writes_total Total number of attempted write requests.
			# TYPE mimir_continuous_test_writes_total counter
			mimir_continuous_test_writes_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_writes_failed_total Total number of failed write requests.
			# TYPE mimir_continuous_test_writes_failed_total counter
			mimir_continuous_test_writes_failed_total{status_code="500",test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_total Total number of attempted query requests.
			# TYPE mimir_continuous_test_queries_total counter
			mimir_continuous_test_queries_total{test="write-read-series"} %d
		`, 1*multiplier, 1*multiplier, 0*multiplier)), "mimir_continuous_test_writes_total", "mimir_continuous_test_writes_failed_total", "mimir_continuous_test_queries_total"))
	})

	t.Run("should keep remote writing next intervals on 4xx error", func(t *testing.T) {
		client := &ClientMock{}
		client.On("WriteSeries", mock.Anything, mock.Anything).Return(400, errors.New("400 error"))

		reg := prometheus.NewPedanticRegistry()
		test := NewWriteReadSeriesTest(cfg, client, logger, reg)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			records.lastWrittenTimestamp = time.Unix(940, 0)
		}
		now := time.Unix(1000, 0)
		err := test.Run(context.Background(), now)
		// An error is expected for smoke-test mode, but we don't want to stop the test.
		assert.Error(t, err)

		client.AssertNumberOfCalls(t, "WriteSeries", 3*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)

			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, time.Unix(960, 0), 2))
			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, time.Unix(980, 0), 2))
			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, time.Unix(1000, 0), 2))
			assert.Equal(t, int64(1000), records.lastWrittenTimestamp.Unix())
		}

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(fmt.Sprintf(`
			# HELP mimir_continuous_test_writes_total Total number of attempted write requests.
			# TYPE mimir_continuous_test_writes_total counter
			mimir_continuous_test_writes_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_writes_failed_total Total number of failed write requests.
			# TYPE mimir_continuous_test_writes_failed_total counter
			mimir_continuous_test_writes_failed_total{status_code="400",test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_total Total number of attempted query requests.
			# TYPE mimir_continuous_test_queries_total counter
			mimir_continuous_test_queries_total{test="write-read-series"} %d
		`, 3*multiplier, 3*multiplier, 0*multiplier)), "mimir_continuous_test_writes_total", "mimir_continuous_test_writes_failed_total", "mimir_continuous_test_queries_total"))
	})

	t.Run("should query written series, compare results and track no failure if results match", func(t *testing.T) {
		now := time.Unix(1000, 0)

		client := &ClientMock{}
		client.On("WriteSeries", mock.Anything, mock.Anything).Return(200, nil)

		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(model.Matrix{
				{Values: []model.SamplePair{newSamplePair(now, tt.generateValue(now)*float64(cfg.NumSeries))}},
			}, nil)
			client.On("Query", mock.Anything, tt.querySum(tt.metricName), mock.Anything, mock.Anything).Return(model.Vector{
				{Timestamp: model.Time(now.UnixMilli()), Value: model.SampleValue(tt.generateValue(now) * float64(cfg.NumSeries))},
			}, nil)
		}

		reg := prometheus.NewPedanticRegistry()
		test := NewWriteReadSeriesTest(cfg, client, logger, reg)

		err := test.Run(context.Background(), now)
		assert.NoError(t, err)

		client.AssertNumberOfCalls(t, "WriteSeries", 1*multiplier)
		client.AssertNumberOfCalls(t, "QueryRange", 4*multiplier)
		client.AssertNumberOfCalls(t, "Query", 4*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)

			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, now, 2))
			assert.Equal(t, int64(1000), records.lastWrittenTimestamp.Unix())

			client.AssertCalled(t, "QueryRange", mock.Anything, tt.querySum(tt.metricName), time.Unix(1000, 0), time.Unix(1000, 0), writeInterval, mock.Anything)

			client.AssertCalled(t, "Query", mock.Anything, tt.querySum(tt.metricName), time.Unix(1000, 0), mock.Anything)
		}

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(fmt.Sprintf(`
			# HELP mimir_continuous_test_writes_total Total number of attempted write requests.
			# TYPE mimir_continuous_test_writes_total counter
			mimir_continuous_test_writes_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_total Total number of attempted query requests.
			# TYPE mimir_continuous_test_queries_total counter
			mimir_continuous_test_queries_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_failed_total Total number of failed query requests.
			# TYPE mimir_continuous_test_queries_failed_total counter
			mimir_continuous_test_queries_failed_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_query_result_checks_total Total number of query results checked for correctness.
			# TYPE mimir_continuous_test_query_result_checks_total counter
			mimir_continuous_test_query_result_checks_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_query_result_checks_failed_total Total number of query results failed when checking for correctness.
			# TYPE mimir_continuous_test_query_result_checks_failed_total counter
			mimir_continuous_test_query_result_checks_failed_total{test="write-read-series"} %d
		`, 1*multiplier, 8*multiplier, 0*multiplier, 8*multiplier, 0*multiplier)),
			"mimir_continuous_test_writes_total", "mimir_continuous_test_writes_failed_total",
			"mimir_continuous_test_queries_total", "mimir_continuous_test_queries_failed_total",
			"mimir_continuous_test_query_result_checks_total", "mimir_continuous_test_query_result_checks_failed_total"))
	})

	t.Run("should query written series, compare results and track failure if results don't match", func(t *testing.T) {
		now := time.Unix(1000, 0)

		client := &ClientMock{}
		client.On("WriteSeries", mock.Anything, mock.Anything).Return(200, nil)
		client.On("QueryRange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(model.Matrix{
			{Values: []model.SamplePair{{Timestamp: model.Time(now.UnixMilli()), Value: 12345}}},
		}, nil)

		client.On("Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(model.Vector{
			{Timestamp: model.Time(now.UnixMilli()), Value: 12345},
		}, nil)

		reg := prometheus.NewPedanticRegistry()
		test := NewWriteReadSeriesTest(cfg, client, logger, reg)

		err := test.Run(context.Background(), now)
		assert.Error(t, err)

		client.AssertNumberOfCalls(t, "WriteSeries", 1*multiplier)
		client.AssertNumberOfCalls(t, "QueryRange", 4*multiplier)
		client.AssertNumberOfCalls(t, "Query", 4*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)

			client.AssertCalled(t, "WriteSeries", mock.Anything, tt.generateSeries(tt.metricName, now, 2))
			assert.Equal(t, int64(1000), records.lastWrittenTimestamp.Unix())

			client.AssertCalled(t, "QueryRange", mock.Anything, tt.querySum(tt.metricName), time.Unix(1000, 0), time.Unix(1000, 0), writeInterval, mock.Anything)

			client.AssertCalled(t, "Query", mock.Anything, tt.querySum(tt.metricName), time.Unix(1000, 0), mock.Anything)
		}

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(fmt.Sprintf(`
			# HELP mimir_continuous_test_writes_total Total number of attempted write requests.
			# TYPE mimir_continuous_test_writes_total counter
			mimir_continuous_test_writes_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_total Total number of attempted query requests.
			# TYPE mimir_continuous_test_queries_total counter
			mimir_continuous_test_queries_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_queries_failed_total Total number of failed query requests.
			# TYPE mimir_continuous_test_queries_failed_total counter
			mimir_continuous_test_queries_failed_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_query_result_checks_total Total number of query results checked for correctness.
			# TYPE mimir_continuous_test_query_result_checks_total counter
			mimir_continuous_test_query_result_checks_total{test="write-read-series"} %d

			# HELP mimir_continuous_test_query_result_checks_failed_total Total number of query results failed when checking for correctness.
			# TYPE mimir_continuous_test_query_result_checks_failed_total counter
			mimir_continuous_test_query_result_checks_failed_total{test="write-read-series"} %d
		`, 1*multiplier, 8*multiplier, 0*multiplier, 8*multiplier, 8*multiplier)),
			"mimir_continuous_test_writes_total", "mimir_continuous_test_writes_failed_total",
			"mimir_continuous_test_queries_total", "mimir_continuous_test_queries_failed_total",
			"mimir_continuous_test_query_result_checks_total", "mimir_continuous_test_query_result_checks_failed_total"))
	})
}

func TestWriteReadSeriesTest_Init(t *testing.T) {
	testWriteReadSeriesTest_Init(t, cfgSample, sampleTestTuples)
	testWriteReadSeriesTest_Init(t, cfgHist, histTestTuples)
}

func testWriteReadSeriesTest_Init(t *testing.T, cfg WriteReadSeriesTestConfig, testTuples []WriteReadSeriesTestTuple) {
	logger := log.NewNopLogger()
	multiplier := len(testTuples)
	cfg.MaxQueryAge = 3 * 24 * time.Hour

	now := time.Unix(10*86400, 0)

	t.Run("no previously written samples found", func(t *testing.T) {
		client := &ClientMock{}

		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 1*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Zero(t, records.lastWrittenTimestamp)
			require.Zero(t, records.queryMinTime)
			require.Zero(t, records.queryMaxTime)
		}
	})

	t.Run("previously written data points are in the range [-2h, -1m]", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-2*time.Hour), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 1*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-2*time.Hour), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})

	t.Run("previously written data points are in the range [-36h, -1m]", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-24*time.Hour).Add(writeInterval), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-48*time.Hour).Add(writeInterval), now.Add(-24*time.Hour), writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-36*time.Hour), now.Add(-24*time.Hour), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 2*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-36*time.Hour), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})

	t.Run("previously written data points are in the range [-36h, -1m] but last data point of previous 24h period is missing", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-24*time.Hour).Add(writeInterval), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-48*time.Hour).Add(writeInterval), now.Add(-24*time.Hour), writeInterval, mock.Anything).Return(model.Matrix{{
				// Last data point is missing.
				Values: generateSamplesSum(now.Add(-36*time.Hour), now.Add(-24*time.Hour).Add(-writeInterval), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 2*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-24*time.Hour).Add(writeInterval), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})

	t.Run("previously written data points are in the range [-24h, -1m]", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-24*time.Hour).Add(writeInterval), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-48*time.Hour).Add(writeInterval), now.Add(-24*time.Hour), writeInterval, mock.Anything).Return(model.Matrix{{}}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 2*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-24*time.Hour).Add(writeInterval), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})

	t.Run("the configured query max age is > 24h", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-24*time.Hour).Add(writeInterval), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-48*time.Hour).Add(writeInterval), now.Add(-24*time.Hour), writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-48*time.Hour).Add(writeInterval), now.Add(-24*time.Hour), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-72*time.Hour).Add(writeInterval), now.Add(-48*time.Hour), writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-72*time.Hour).Add(writeInterval), now.Add(-48*time.Hour), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 3*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-72*time.Hour).Add(writeInterval), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})

	t.Run("the configured query max age is < 24h", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-2*time.Hour), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-2*time.Hour), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
		}

		testCfg := cfg
		testCfg.MaxQueryAge = 2 * time.Hour
		test := NewWriteReadSeriesTest(testCfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 1*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-2*time.Hour), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})

	t.Run("the most recent previously written data point is older than 1h ago", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-2*time.Hour).Add(writeInterval), now.Add(-1*time.Hour), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 1*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Zero(t, records.lastWrittenTimestamp)
			require.Zero(t, records.queryMinTime)
			require.Zero(t, records.queryMaxTime)
		}
	})

	t.Run("the first query fails", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{}, errors.New("failed"))
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 1*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Zero(t, records.lastWrittenTimestamp)
			require.Zero(t, records.queryMinTime)
			require.Zero(t, records.queryMaxTime)
		}
	})

	t.Run("a subsequent query fails", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-24*time.Hour).Add(writeInterval), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-48*time.Hour).Add(writeInterval), now.Add(-24*time.Hour), writeInterval, mock.Anything).Return(model.Matrix{{}}, errors.New("failed"))
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 2*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-24*time.Hour).Add(writeInterval), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})

	t.Run("the testing tool has been restarted with a different number of series in the middle of the last 24h period", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: append(
					generateSamplesSum(now.Add(-24*time.Hour).Add(writeInterval), now.Add(-67*time.Minute), cfg.NumSeries-1, writeInterval, tt.generateValue),
					generateSamplesSum(now.Add(-67*time.Minute).Add(writeInterval), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue)...,
				),
			}}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 1*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-67*time.Minute).Add(writeInterval), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})

	t.Run("the testing tool has been restarted with a different number of series in the middle of the previous 24h period", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-24*time.Hour).Add(writeInterval), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-48*time.Hour).Add(writeInterval), now.Add(-24*time.Hour), writeInterval, mock.Anything).Return(model.Matrix{{
				Values: append(
					generateSamplesSum(now.Add(-48*time.Hour).Add(writeInterval), now.Add(-36*time.Hour).Add(time.Minute), cfg.NumSeries-1, writeInterval, tt.generateValue),
					generateSamplesSum(now.Add(-36*time.Hour).Add(time.Minute+writeInterval), now.Add(-24*time.Hour), cfg.NumSeries, writeInterval, tt.generateValue)...,
				),
			}}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 2*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-36*time.Hour).Add(time.Minute+writeInterval), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})

	t.Run("the testing tool has been restarted with a different number of series exactly at the beginning of this 24h period", func(t *testing.T) {
		client := &ClientMock{}
		for _, tt := range testTuples {
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-24*time.Hour).Add(writeInterval), now, writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-24*time.Hour).Add(writeInterval), now.Add(-1*time.Minute), cfg.NumSeries, writeInterval, tt.generateValue),
			}}, nil)
			client.On("QueryRange", mock.Anything, tt.querySum(tt.metricName), now.Add(-48*time.Hour).Add(writeInterval), now.Add(-24*time.Hour), writeInterval, mock.Anything).Return(model.Matrix{{
				Values: generateSamplesSum(now.Add(-24*time.Hour).Add(writeInterval), now.Add(-1*time.Minute), cfg.NumSeries-1, writeInterval, tt.generateValue),
			}}, nil)
		}

		test := NewWriteReadSeriesTest(cfg, client, logger, nil)

		require.NoError(t, test.Init(context.Background(), now))

		client.AssertNumberOfCalls(t, "QueryRange", 2*multiplier)

		for _, tt := range testTuples {
			records := tt.getMetricHistory(test)
			require.Equal(t, now.Add(-1*time.Minute), records.lastWrittenTimestamp)
			require.Equal(t, now.Add(-24*time.Hour).Add(writeInterval), records.queryMinTime)
			require.Equal(t, now.Add(-1*time.Minute), records.queryMaxTime)
		}
	})
}

func TestWriteReadSeriesTest_getRangeQueryTimeRanges(t *testing.T) {
	cfg := WriteReadSeriesTestConfig{}
	flagext.DefaultValues(&cfg)
	cfg.MaxQueryAge = 2 * 24 * time.Hour

	now := time.Unix(int64((10*24*time.Hour)+(2*time.Second)), 0)

	t.Run("min/max query time has not been set yet", func(t *testing.T) {
		test := NewWriteReadSeriesTest(cfg, &ClientMock{}, log.NewNopLogger(), nil)

		actualRanges, actualInstants, err := test.getQueryTimeRanges(now, &test.sampleMetric)
		assert.Error(t, err)
		assert.Empty(t, actualRanges)
		assert.Empty(t, actualInstants)
	})

	t.Run("min/max query time is older than max age", func(t *testing.T) {
		test := NewWriteReadSeriesTest(cfg, &ClientMock{}, log.NewNopLogger(), nil)
		test.sampleMetric.queryMinTime = now.Add(-cfg.MaxQueryAge).Add(-time.Minute)
		test.sampleMetric.queryMaxTime = now.Add(-cfg.MaxQueryAge).Add(-time.Minute)

		actualRanges, actualInstants, err := test.getQueryTimeRanges(now, &test.sampleMetric)
		assert.Error(t, err)
		assert.Empty(t, actualRanges)
		assert.Empty(t, actualInstants)
	})

	t.Run("min query time = max query time", func(t *testing.T) {
		test := NewWriteReadSeriesTest(cfg, &ClientMock{}, log.NewNopLogger(), nil)
		test.sampleMetric.queryMinTime = now.Add(-time.Minute)
		test.sampleMetric.queryMaxTime = now.Add(-time.Minute)

		actualRanges, actualInstants, err := test.getQueryTimeRanges(now, &test.sampleMetric)
		require.NoError(t, err)

		require.Len(t, actualRanges, 2)
		require.Equal(t, [2]time.Time{now.Add(-time.Minute), now.Add(-time.Minute)}, actualRanges[0]) // Last 1h.
		require.Equal(t, [2]time.Time{now.Add(-time.Minute), now.Add(-time.Minute)}, actualRanges[1]) // Random time range.

		require.Len(t, actualInstants, 2)
		require.Equal(t, now.Add(-time.Minute), actualInstants[0]) // Last 1h.
		require.Equal(t, now.Add(-time.Minute), actualInstants[1]) // Random time range.
	})

	t.Run("min and max query time are within the last 1h", func(t *testing.T) {
		test := NewWriteReadSeriesTest(cfg, &ClientMock{}, log.NewNopLogger(), nil)
		test.sampleMetric.queryMinTime = now.Add(-30 * time.Minute)
		test.sampleMetric.queryMaxTime = now.Add(-time.Minute)

		actualRanges, actualInstants, err := test.getQueryTimeRanges(now, &test.sampleMetric)
		require.NoError(t, err)
		require.Len(t, actualRanges, 2)
		require.Equal(t, [2]time.Time{now.Add(-30 * time.Minute), now.Add(-time.Minute)}, actualRanges[0]) // Last 1h.

		require.Len(t, actualInstants, 2)
		require.Equal(t, now.Add(-time.Minute), actualInstants[0]) // Last 1h.

		// Random time range.
		require.GreaterOrEqual(t, actualRanges[len(actualRanges)-1][0].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualRanges[len(actualRanges)-1][1].Unix(), test.sampleMetric.queryMaxTime.Unix())

		require.GreaterOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMaxTime.Unix())
	})

	t.Run("min and max query time are within the last 2h", func(t *testing.T) {
		test := NewWriteReadSeriesTest(cfg, &ClientMock{}, log.NewNopLogger(), nil)
		test.sampleMetric.queryMinTime = now.Add(-90 * time.Minute)
		test.sampleMetric.queryMaxTime = now.Add(-80 * time.Minute)

		actualRanges, actualInstants, err := test.getQueryTimeRanges(now, &test.sampleMetric)
		require.NoError(t, err)
		require.Len(t, actualRanges, 2)
		require.Equal(t, [2]time.Time{now.Add(-90 * time.Minute), now.Add(-80 * time.Minute)}, actualRanges[0]) // Last 24h.

		require.Len(t, actualInstants, 2)
		require.Equal(t, now.Add(-90*time.Minute), actualInstants[0]) // Last 24h.

		// Random time range.
		require.GreaterOrEqual(t, actualRanges[len(actualRanges)-1][0].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualRanges[len(actualRanges)-1][1].Unix(), test.sampleMetric.queryMaxTime.Unix())

		require.GreaterOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMaxTime.Unix())
	})

	t.Run("min query time is older than 24h", func(t *testing.T) {
		test := NewWriteReadSeriesTest(cfg, &ClientMock{}, log.NewNopLogger(), nil)
		test.sampleMetric.queryMinTime = now.Add(-30 * time.Hour)
		test.sampleMetric.queryMaxTime = now.Add(-time.Minute)

		actualRanges, actualInstants, err := test.getQueryTimeRanges(now, &test.sampleMetric)
		require.NoError(t, err)
		require.Len(t, actualRanges, 4)
		require.Equal(t, [2]time.Time{now.Add(-time.Hour), now.Add(-time.Minute)}, actualRanges[0])         // Last 1h.
		require.Equal(t, [2]time.Time{now.Add(-24 * time.Hour), now.Add(-time.Minute)}, actualRanges[1])    // Last 24h.
		require.Equal(t, [2]time.Time{now.Add(-24 * time.Hour), now.Add(-23 * time.Hour)}, actualRanges[2]) // From last 23h to last 24h.

		require.Len(t, actualInstants, 3)
		require.Equal(t, now.Add(-time.Minute), actualInstants[0])  // Last 1h.
		require.Equal(t, now.Add(-24*time.Hour), actualInstants[1]) // Last 24h.

		// Random time range.
		require.GreaterOrEqual(t, actualRanges[len(actualRanges)-1][0].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualRanges[len(actualRanges)-1][1].Unix(), test.sampleMetric.queryMaxTime.Unix())

		require.GreaterOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMaxTime.Unix())
	})

	t.Run("max query time is older than 24h but more recent than max query age", func(t *testing.T) {
		test := NewWriteReadSeriesTest(cfg, &ClientMock{}, log.NewNopLogger(), nil)
		test.sampleMetric.queryMinTime = now.Add(-30 * time.Hour)
		test.sampleMetric.queryMaxTime = now.Add(-25 * time.Hour)

		actualRanges, actualInstants, err := test.getQueryTimeRanges(now, &test.sampleMetric)
		require.NoError(t, err)
		require.Len(t, actualRanges, 1)
		require.Len(t, actualInstants, 1)

		// Random time range.
		require.GreaterOrEqual(t, actualRanges[len(actualRanges)-1][0].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualRanges[len(actualRanges)-1][1].Unix(), test.sampleMetric.queryMaxTime.Unix())

		require.GreaterOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMaxTime.Unix())
	})

	t.Run("min query time is older than 24h but max query age is only 10m", func(t *testing.T) {
		cfg := cfg
		cfg.MaxQueryAge = 10 * time.Minute

		test := NewWriteReadSeriesTest(cfg, &ClientMock{}, log.NewNopLogger(), nil)
		test.sampleMetric.queryMinTime = now.Add(-30 * time.Hour)
		test.sampleMetric.queryMaxTime = now.Add(-time.Minute)

		actualRanges, actualInstants, err := test.getQueryTimeRanges(now, &test.sampleMetric)
		require.NoError(t, err)
		require.Len(t, actualRanges, 2)
		require.Equal(t, [2]time.Time{now.Add(-10 * time.Minute), now.Add(-time.Minute)}, actualRanges[0]) // Last 1h.

		require.Len(t, actualInstants, 2)
		require.Equal(t, now.Add(-time.Minute), actualInstants[0]) // Last 1h.

		// Random time range.
		require.GreaterOrEqual(t, actualRanges[len(actualRanges)-1][0].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualRanges[len(actualRanges)-1][1].Unix(), test.sampleMetric.queryMaxTime.Unix())

		require.GreaterOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMinTime.Unix())
		require.LessOrEqual(t, actualInstants[len(actualInstants)-1].Unix(), test.sampleMetric.queryMaxTime.Unix())
	})
}

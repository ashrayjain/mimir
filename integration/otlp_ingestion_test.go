// SPDX-License-Identifier: AGPL-3.0-only
//go:build requires_docker

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/e2e"
	e2edb "github.com/grafana/e2e/db"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/mimir/integration/e2emimir"
)

func TestOTLPIngestion(t *testing.T) {
	s, err := e2e.NewScenario(networkName)
	require.NoError(t, err)
	defer s.Close()

	// Start dependencies.
	minio := e2edb.NewMinio(9000, blocksBucketName)
	require.NoError(t, s.StartAndWaitReady(minio))

	// Start Mimir components.
	require.NoError(t, copyFileToSharedDir(s, "docs/configurations/single-process-config-blocks.yaml", mimirConfigFile))

	// Start Mimir in single binary mode, reading the config from file and overwriting
	// the backend config to make it work with Minio.
	flags := map[string]string{
		"-blocks-storage.s3.access-key-id":     e2edb.MinioAccessKey,
		"-blocks-storage.s3.secret-access-key": e2edb.MinioSecretKey,
		"-blocks-storage.s3.bucket-name":       blocksBucketName,
		"-blocks-storage.s3.endpoint":          fmt.Sprintf("%s-minio-9000:9000", networkName),
		"-blocks-storage.s3.insecure":          "true",
	}

	mimir := e2emimir.NewSingleBinary("mimir-1", flags, e2emimir.WithConfigFile(mimirConfigFile), e2emimir.WithPorts(9009, 9095))
	require.NoError(t, s.StartAndWaitReady(mimir))

	c, err := e2emimir.NewClient(mimir.HTTPEndpoint(), mimir.HTTPEndpoint(), "", "", "user-1")
	require.NoError(t, err)

	// Push some series to Mimir.
	now := time.Now()
	series, expectedVector, expectedMatrix := generateSeries("series_1", now, prompb.Label{Name: "foo", Value: "bar"})

	res, err := c.PushOTLP(series)
	require.NoError(t, err)
	require.Equal(t, 200, res.StatusCode)

	// Query the series.
	result, err := c.Query("series_1", now)
	require.NoError(t, err)
	require.Equal(t, model.ValVector, result.Type())
	assert.Equal(t, expectedVector, result.(model.Vector))

	labelValues, err := c.LabelValues("foo", prometheusMinTime, prometheusMaxTime, nil)
	require.NoError(t, err)
	require.Equal(t, model.LabelValues{"bar"}, labelValues)

	labelNames, err := c.LabelNames(prometheusMinTime, prometheusMaxTime)
	require.NoError(t, err)
	require.Equal(t, []string{"__name__", "foo"}, labelNames)

	rangeResult, err := c.QueryRange("series_1", now.Add(-15*time.Minute), now, 15*time.Second)
	require.NoError(t, err)
	require.Equal(t, model.ValMatrix, rangeResult.Type())
	require.Equal(t, expectedMatrix, rangeResult.(model.Matrix))

	// Push series with histograms to Mimir
	series = []prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "foo", Value: "bar"},
			},
			Histograms: []prompb.Histogram{
				remote.HistogramToHistogramProto(1337, tsdb.GenerateTestHistograms(1)[0]),
			},
		},
	}
	_, err = c.PushOTLP(series)
	require.NoError(t, err)
	// TODO query and assert series that contain native histogram once querying
	// of native histograms is supported
}

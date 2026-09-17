package catalog

import (
	"fmt"
	"strings"
)

// ParseRegistryChartRef translates an external registry's single-string chartRef +
// chartVersion (contracts/module-registry-protocol.md) into the structured ChartRef
// booth-core's actual install API requires (chart: {path, repoUrl, chartName,
// version} — see coreclient.InstallRequest).
//
// FLAGGED GAP (see docs/decisions/0001-chartref-translation-gap.md): the protocol
// document says chartRef/chartVersion are "passed verbatim" to booth-core's install
// API and that "booth-module-store doesn't interpret or validate the chart itself
// beyond passing it through." That's not actually possible — booth-core's implemented
// API (internal/api/server.go's installRequest) has never accepted a single opaque
// string, only the structured RepoURL/ChartName/Version/Path fields. Something has to
// parse chartRef into those fields, and the protocol doesn't say how.
//
// This function is a narrow, clearly-scoped interim answer, not a claim that the
// mismatch is resolved: it only understands "oci://host/path/chart-name" references
// (splitting the last path segment off as the chart name, the rest as the repo URL),
// which covers the protocol's own example. Any other shape is rejected with an error
// rather than guessed at, so a bad assumption fails loudly at install time instead of
// silently installing the wrong chart.
func ParseRegistryChartRef(chartRef, chartVersion string) (ChartRef, error) {
	if chartRef == "" {
		return ChartRef{}, fmt.Errorf("chartRef is empty")
	}
	if !strings.HasPrefix(chartRef, "oci://") {
		return ChartRef{}, fmt.Errorf("chartRef %q: only oci:// references are understood by this v0 interim translation (see docs/decisions/0001-chartref-translation-gap.md)", chartRef)
	}

	trimmed := strings.TrimSuffix(chartRef, "/")
	lastSlash := strings.LastIndex(trimmed, "/")
	if lastSlash < len("oci://") {
		return ChartRef{}, fmt.Errorf("chartRef %q: expected oci://host/path/chart-name", chartRef)
	}

	repoURL := trimmed[:lastSlash]
	chartName := trimmed[lastSlash+1:]
	if chartName == "" {
		return ChartRef{}, fmt.Errorf("chartRef %q: empty chart name after last '/'", chartRef)
	}

	return ChartRef{
		RepoURL:   repoURL,
		ChartName: chartName,
		Version:   chartVersion,
	}, nil
}

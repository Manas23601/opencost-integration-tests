package count

// Description - Checks for the aggregate count of pods for each namespace from prometheus request
// and allocation API request are the same

// Both prometheus and allocation seem to be returning duplicate results. Does this we might be double counting costs times?

import (
	// "fmt"
	"github.com/opencost/opencost-integration-tests/pkg/api"
	"github.com/opencost/opencost-integration-tests/pkg/prometheus"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestQueryAllocation(t *testing.T) {
	apiObj := api.NewAPI()

	testCases := []struct {
		name       string
		window     string
		aggregate  string
		accumulate string
	}{
		{
			name:       "Yesterday",
			window:     "24h",
			aggregate:  "pod",
			accumulate: "false",
		},
	}

	t.Logf("testCases: %v", testCases)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// API Client
			apiResponse, err := apiObj.GetAllocation(api.AllocationRequest{
				Window:     tc.window,
				Aggregate:  tc.aggregate,
				Accumulate: tc.accumulate,
			})

			if err != nil {
				t.Fatalf("Error while calling Allocation API %v", err)
			}
			if apiResponse.Code != 200 {
				t.Errorf("API returned non-200 code")
			}

			queryEnd := time.Now().UTC().Truncate(time.Hour).Add(time.Hour)
			endTime := queryEnd.Unix()

			// Prometheus Client
			// Want to Run avg(avg_over_time(kube_pod_container_status_running[24h]) != 0) by (container, pod, namespace)
			// Running avg(avg_over_time(kube_pod_container_status_running[24h])) by (container, pod, namespace)
			client := prometheus.NewClient()
			promInput := prometheus.PrometheusInput{
				Metric: "kube_pod_container_status_running",
				// MetricNotEqualTo: "0",
				Function:    []string{"avg_over_time", "avg"},
				QueryWindow: tc.window,
				AggregateBy: []string{"container", "pod", "namespace"},
				Time:        &endTime,
			}

			promResponse, err := client.RunPromQLQuery(promInput)
			if err != nil {
				t.Fatalf("Error while calling Prometheus API %v", err)
			}

			// Calculate Number of Pods per Aggregate for API Object
			type podAggregation struct {
				Pods []string
			}
			// Namespace based calculation
			var apiAggregateCount = make(map[string]*podAggregation)

			for pod, allocationResponeItem := range apiResponse.Data[0] {
				// Synthetic value generated and returned by /allocation and not /prometheus
				if slices.Contains([]string{"prometheus-system-unmounted-pvcs", "network-load-gen-unmounted-pvcs"}, pod) {
					continue
				}
				podNamespace := allocationResponeItem.Properties.Namespace
				apiAggregateItem, namespacePresent := apiAggregateCount[podNamespace]
				if !namespacePresent {
					apiAggregateCount[podNamespace] = &podAggregation{
						Pods: []string{pod},
					}
					continue
				}
				if allocationResponeItem.Properties.Pod == "" {
					continue
				}
				if !slices.Contains(apiAggregateItem.Pods, pod) {
					apiAggregateItem.Pods = append(apiAggregateItem.Pods, pod)
				}
			}

			// Calculate Number of Pods per Aggregate for Prom Object
			var promAggregateCount = make(map[string]*podAggregation)

			for _, metric := range promResponse.Data.Result {
				podNamespace := metric.Metric.Namespace
				pod := metric.Metric.Pod
				// This pod was down, unable to do it with the query
				if metric.Value.Value == 0 {
					continue
				}
				promAggregateItem, namespacePresent := promAggregateCount[podNamespace]
				if !namespacePresent {
					promAggregateCount[podNamespace] = &podAggregation{
						Pods: []string{pod},
					}
					continue
				}
				if !slices.Contains(promAggregateItem.Pods, pod) {
					promAggregateItem.Pods = append(promAggregateItem.Pods, pod)
				}
			}

			if len(promAggregateCount) != len(apiAggregateCount) {
				t.Logf("Namespace Count Allocation %d != Prometheus %d", len(apiAggregateCount), len(promAggregateCount))
			}
			for namespace, _ := range promAggregateCount {
				apiNamespaceCount, apiNamespacePresent := apiAggregateCount[namespace]
				promNamespaceCount, promNamespacePresent := promAggregateCount[namespace]
				if apiNamespacePresent && promNamespacePresent {
					t.Logf("Namespace: %s", namespace)
					sort.Strings(apiNamespaceCount.Pods)
					sort.Strings(promNamespaceCount.Pods)
					if len(apiNamespaceCount.Pods) != len(promNamespaceCount.Pods) {
						t.Errorf("[Fail]: /allocation (%d) != Prometheus (%d)", len(apiNamespaceCount.Pods), len(promNamespaceCount.Pods))
						t.Errorf("API Pods:\n - %v\nPrometheus Pods:\n - %v", strings.Join(apiNamespaceCount.Pods, "\n - "), strings.Join(promNamespaceCount.Pods, "\n - "))
					} else {
						t.Logf("[Pass]: Pod Count %d", len(apiNamespaceCount.Pods))
					}
				} else {
					t.Errorf("Namespace Missing: Prometheus(%v), allocation API(%v)", apiNamespacePresent, promNamespacePresent)
				}
			}
		})
	}
}

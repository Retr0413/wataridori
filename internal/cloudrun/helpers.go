package cloudrun

import (
	"errors"
	"strings"

	runpb "cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/api/iterator"
)

// shortName strips the resource-name prefix, keeping the last segment.
func shortName(resource string) string {
	if i := strings.LastIndex(resource, "/"); i >= 0 {
		return resource[i+1:]
	}
	return resource
}

func containerImage(containers []*runpb.Container) string {
	if len(containers) == 0 {
		return ""
	}
	return containers[0].GetImage()
}

func trafficByRevision(svc *runpb.Service) map[string]int32 {
	traffic := map[string]int32{}
	latest := shortName(svc.GetLatestReadyRevision())
	for _, ts := range svc.GetTrafficStatuses() {
		rev := ts.GetRevision()
		if rev == "" {
			rev = latest
		}
		traffic[shortName(rev)] += ts.GetPercent()
	}
	return traffic
}

func conditionReady(conditions []*runpb.Condition) bool {
	for _, c := range conditions {
		if c.GetType() == "Ready" {
			return c.GetState() == runpb.Condition_CONDITION_SUCCEEDED
		}
	}
	return false
}

// conditionReadyService: a service is ready when its terminal condition
// (reconciliation) succeeded.
func conditionReadyService(svc *runpb.Service) bool {
	tc := svc.GetTerminalCondition()
	return tc.GetState() == runpb.Condition_CONDITION_SUCCEEDED
}

func conditionMessage(conditions []*runpb.Condition) string {
	for _, c := range conditions {
		if c.GetState() != runpb.Condition_CONDITION_SUCCEEDED && c.GetMessage() != "" {
			return c.GetMessage()
		}
	}
	return ""
}

func isIteratorDone(err error) bool {
	return errors.Is(err, iterator.Done)
}

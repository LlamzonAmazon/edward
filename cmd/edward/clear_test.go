package main

import (
	"reflect"
	"testing"

	"github.com/LlamzonAmazon/edward/internal/store"
)

func TestPlanClear(t *testing.T) {
	findings := []store.Finding{
		{ID: "exposed-rapportd|3722|u", Value: "rapportd on port 3722"},
		{ID: "unknownAddress-Notes|993|u", Value: "Notes → a:993"},
		{ID: "unknownAddress-Notes|993|u", Value: "Notes → b:993"},
		{ID: "exposed-Live|20909|u", Value: "Live on port 20909"},
	}
	set, skipped, missing := planClear(findings, []string{
		"rapportd on port 3722",
		"Notes → a:993",
		"rapportd on port 49739",
	})
	wantSet := []idValues{{"exposed-rapportd|3722|u", []string{"rapportd on port 3722"}}}
	if !reflect.DeepEqual(set, wantSet) {
		t.Fatalf("set = %v, want %v", set, wantSet)
	}
	wantSkipped := []idValues{{"unknownAddress-Notes|993|u", []string{"Notes → b:993"}}}
	if !reflect.DeepEqual(skipped, wantSkipped) {
		t.Fatalf("skipped = %v, want %v", skipped, wantSkipped)
	}
	if !reflect.DeepEqual(missing, []string{"rapportd on port 49739"}) {
		t.Fatalf("missing = %v", missing)
	}
}

func TestPlanClearSharedIDAllApproved(t *testing.T) {
	findings := []store.Finding{
		{ID: "n", Value: "Notes → a:993"},
		{ID: "n", Value: "Notes → b:993"},
	}
	set, skipped, missing := planClear(findings, []string{"Notes → a:993", "Notes → b:993"})
	if len(set) != 1 || len(set[0].Values) != 2 || skipped != nil || missing != nil {
		t.Fatalf("set=%v skipped=%v missing=%v", set, skipped, missing)
	}
}

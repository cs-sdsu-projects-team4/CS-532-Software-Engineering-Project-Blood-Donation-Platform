package domain

import (
	"strings"
	"testing"
)

// fullAnswers returns a complete answer set with every question answered in a
// non-flagging way, which callers then tweak per test.
func fullAnswers() map[string]bool {
	a := map[string]bool{}
	for _, q := range Questionnaire() {
		a[q.ID] = !q.FlagAnswer // answer opposite of the flag value → no flags
	}
	return a
}

func TestEvaluateScreening_NoFlags(t *testing.T) {
	res, missing, ok := EvaluateScreening(fullAnswers())
	if !ok {
		t.Fatalf("expected ok, missing=%v", missing)
	}
	if res.HasFlags {
		t.Errorf("expected no flags, got %v", res.FlaggedQuestionIDs)
	}
	if len(res.FlaggedQuestionIDs) != 0 {
		t.Errorf("expected empty flag list, got %v", res.FlaggedQuestionIDs)
	}
}

func TestEvaluateScreening_Flagged(t *testing.T) {
	a := fullAnswers()
	a["donated_last_8_weeks"] = true // this is the flag answer
	res, _, ok := EvaluateScreening(a)
	if !ok {
		t.Fatal("expected ok")
	}
	if !res.HasFlags {
		t.Fatal("expected flags")
	}
	found := false
	for _, id := range res.FlaggedQuestionIDs {
		if id == "donated_last_8_weeks" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected donated_last_8_weeks flagged, got %v", res.FlaggedQuestionIDs)
	}
}

// FR-04: guidance must be non-diagnostic and defer to the clinic.
func TestEvaluateScreening_GuidanceIsNonDiagnostic(t *testing.T) {
	a := fullAnswers()
	a["recent_illness_or_fever"] = true
	res, _, _ := EvaluateScreening(a)

	lower := strings.ToLower(res.Guidance)
	if !strings.Contains(lower, "clinic") {
		t.Error("guidance must direct the donor to the clinic")
	}
	// It must explicitly disclaim being a diagnosis (FR-04)...
	if !strings.Contains(lower, "not a medical diagnosis") {
		t.Errorf("guidance must disclaim being a diagnosis, got %q", res.Guidance)
	}
	// ...and must never state a final, decisive eligibility outcome.
	for _, banned := range []string{"you are ineligible", "you cannot donate", "disqualified", "you may not donate"} {
		if strings.Contains(lower, banned) {
			t.Errorf("guidance must avoid decisive wording, found %q", banned)
		}
	}
}

func TestEvaluateScreening_Incomplete(t *testing.T) {
	a := fullAnswers()
	delete(a, "feeling_well_today")
	_, missing, ok := EvaluateScreening(a)
	if ok {
		t.Fatal("expected incomplete submission to fail")
	}
	if len(missing) != 1 || missing[0] != "feeling_well_today" {
		t.Errorf("expected missing [feeling_well_today], got %v", missing)
	}
}

func TestIsScreeningQuestionID(t *testing.T) {
	if !IsScreeningQuestionID("donated_last_8_weeks") {
		t.Error("known id should be recognized")
	}
	if IsScreeningQuestionID("made_up") {
		t.Error("unknown id should be rejected")
	}
}

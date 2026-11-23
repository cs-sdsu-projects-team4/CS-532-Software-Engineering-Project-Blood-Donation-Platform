package domain

// Preliminary donor screening (FR-03, FR-04).
//
// This questionnaire is a lightweight, self-reported pre-check that helps donors
// understand whether a temporary restriction MIGHT apply. It is deliberately
// NOT diagnostic and never decides eligibility: the clinic verifies identity,
// repeats required screening/testing, and makes the final decision in person.
// We store only the self-reported yes/no answers — never medical test records
// (NFR-02).

// ScreeningQuestion is one yes/no item in the preliminary questionnaire.
type ScreeningQuestion struct {
	ID     string `json:"id"`
	Prompt string `json:"prompt"`
	// FlagAnswer is the answer that suggests a POSSIBLE temporary restriction
	// worth clinic review. For most risk questions that is "yes"; for the
	// feeling-well question it is "no".
	FlagAnswer bool `json:"flag_answer"`
}

// nonDiagnosticGuidance is shown whenever any answer is flagged. It avoids
// diagnosis and defers the final decision to the clinic (FR-04).
const nonDiagnosticGuidance = "Some of your answers suggest a temporary restriction may apply. " +
	"This is general guidance, not a medical diagnosis or a decision about your eligibility. " +
	"You can still proceed; the clinic will review your information and make the final decision " +
	"in person before any donation."

// noFlagsGuidance is shown when nothing is flagged — still non-committal.
const noFlagsGuidance = "No preliminary restrictions were flagged from your answers. " +
	"This is not a guarantee of eligibility; the clinic makes the final decision in person."

// questions is the canonical preliminary questionnaire. Wording is intentionally
// general and should be reviewed by a medical adviser before production use.
var questions = []ScreeningQuestion{
	{ID: "age_18_or_older", Prompt: "Are you 18 years of age or older?", FlagAnswer: false},
	{ID: "feeling_well_today", Prompt: "Are you feeling well and healthy today?", FlagAnswer: false},
	{ID: "donated_last_8_weeks", Prompt: "Have you donated blood in the last 8 weeks?", FlagAnswer: true},
	{ID: "recent_tattoo_or_piercing", Prompt: "Have you had a tattoo or piercing in the last few months?", FlagAnswer: true},
	{ID: "recent_illness_or_fever", Prompt: "Have you had a fever or infection in the last two weeks?", FlagAnswer: true},
	{ID: "pregnant_or_recent_birth", Prompt: "Are you currently pregnant or have you given birth in the last six months?", FlagAnswer: true},
}

// Questionnaire returns a copy of the preliminary questions, for the client to
// render (FR-03).
func Questionnaire() []ScreeningQuestion {
	out := make([]ScreeningQuestion, len(questions))
	copy(out, questions)
	return out
}

// ScreeningResult is the outcome of evaluating a submitted questionnaire.
type ScreeningResult struct {
	HasFlags bool   `json:"has_flags"`
	Guidance string `json:"guidance"`
	// FlaggedQuestionIDs lists which questions triggered the guidance, so the
	// client can highlight them. Never a diagnosis — just which items to review.
	FlaggedQuestionIDs []string `json:"flagged_question_ids"`
}

// EvaluateScreening checks that every question is answered and computes whether
// any answer is flagged, returning non-diagnostic guidance (FR-04). It returns
// ok=false with the list of missing question IDs when the submission is
// incomplete, so the handler can return a 400 rather than storing partial data.
func EvaluateScreening(answers map[string]bool) (result ScreeningResult, missing []string, ok bool) {
	for _, q := range questions {
		if _, present := answers[q.ID]; !present {
			missing = append(missing, q.ID)
		}
	}
	if len(missing) > 0 {
		return ScreeningResult{}, missing, false
	}

	flagged := make([]string, 0)
	for _, q := range questions {
		if answers[q.ID] == q.FlagAnswer {
			flagged = append(flagged, q.ID)
		}
	}

	res := ScreeningResult{
		HasFlags:           len(flagged) > 0,
		FlaggedQuestionIDs: flagged,
	}
	if res.HasFlags {
		res.Guidance = nonDiagnosticGuidance
	} else {
		res.Guidance = noFlagsGuidance
	}
	return res, nil, true
}

// IsScreeningQuestionID reports whether id is a known question, so handlers can
// reject unknown keys in a submission.
func IsScreeningQuestionID(id string) bool {
	for _, q := range questions {
		if q.ID == id {
			return true
		}
	}
	return false
}

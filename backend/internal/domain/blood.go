// Package domain holds entities and business rules that must not live in HTTP
// handlers or SQL: blood compatibility, screening evaluation, eligibility. Keep
// this package free of HTTP, database, and framework imports so the rules stay
// unit-testable in isolation.
package domain

// BloodType is a validated ABO/Rh red-cell type. Donors may also report their
// type as "unknown" (see BloodUnknown); requests always name one of the 8.
type BloodType string

const (
	ONeg  BloodType = "O-"
	OPos  BloodType = "O+"
	ANeg  BloodType = "A-"
	APos  BloodType = "A+"
	BNeg  BloodType = "B-"
	BPos  BloodType = "B+"
	ABNeg BloodType = "AB-"
	ABPos BloodType = "AB+"
	// BloodUnknown is a donor self-report only. An unknown donor does NOT pass the
	// deterministic compatibility filter for a type-specific request (see
	// CompatibleDonorTypes' contract and the matching pipeline).
	BloodUnknown BloodType = "unknown"
)

// validTypes is the set of the 8 concrete red-cell types (excludes "unknown").
var validTypes = map[BloodType]bool{
	ONeg: true, OPos: true, ANeg: true, APos: true,
	BNeg: true, BPos: true, ABNeg: true, ABPos: true,
}

// recipientToDonors is the single source of truth for red-cell compatibility:
// "a patient of type KEY can safely receive from these donor types." The matching
// pipeline's first filter uses this. Direction matters — it is not symmetric.
var recipientToDonors = map[BloodType][]BloodType{
	ONeg:  {ONeg},
	OPos:  {ONeg, OPos},
	ANeg:  {ONeg, ANeg},
	APos:  {ONeg, OPos, ANeg, APos},
	BNeg:  {ONeg, BNeg},
	BPos:  {ONeg, OPos, BNeg, BPos},
	ABNeg: {ONeg, ANeg, BNeg, ABNeg},
	ABPos: {ONeg, OPos, ANeg, APos, BNeg, BPos, ABNeg, ABPos},
}

// IsValidBloodType reports whether t is one of the 8 concrete types. "unknown"
// is NOT valid here — validate request/recipient types with this.
func IsValidBloodType(t BloodType) bool { return validTypes[t] }

// IsValidDonorBloodType reports whether t is acceptable for a donor profile:
// one of the 8 concrete types or "unknown" (FR-01).
func IsValidDonorBloodType(t BloodType) bool { return validTypes[t] || t == BloodUnknown }

// CompatibleDonorTypes returns the donor types that may give to a recipient of
// the requested type, and ok=false if the recipient type is not one of the 8.
// The returned slice is a copy callers may freely retain.
func CompatibleDonorTypes(recipient BloodType) (donors []BloodType, ok bool) {
	src, found := recipientToDonors[recipient]
	if !found {
		return nil, false
	}
	out := make([]BloodType, len(src))
	copy(out, src)
	return out, true
}

// IsCompatible reports whether a donor of donorType may give red cells to a
// recipient of recipientType. An "unknown" (or otherwise invalid) donor type is
// never compatible for a type-specific request — we do not guess. This governs
// matching/outreach only; the clinic verifies type and decides in person.
func IsCompatible(donorType, recipientType BloodType) bool {
	if !IsValidBloodType(donorType) {
		return false
	}
	donors, ok := recipientToDonors[recipientType]
	if !ok {
		return false
	}
	for _, d := range donors {
		if d == donorType {
			return true
		}
	}
	return false
}

package domain

import (
	"sort"
	"testing"
)

// wantDonors is the authoritative recipient→donor matrix, duplicated here so the
// test fails loudly if the production map is ever changed by accident.
var wantDonors = map[BloodType][]BloodType{
	ONeg:  {ONeg},
	OPos:  {ONeg, OPos},
	ANeg:  {ONeg, ANeg},
	APos:  {ONeg, OPos, ANeg, APos},
	BNeg:  {ONeg, BNeg},
	BPos:  {ONeg, OPos, BNeg, BPos},
	ABNeg: {ONeg, ANeg, BNeg, ABNeg},
	ABPos: {ONeg, OPos, ANeg, APos, BNeg, BPos, ABNeg, ABPos},
}

func sortedTypes(ts []BloodType) []BloodType {
	out := append([]BloodType(nil), ts...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func TestCompatibleDonorTypes(t *testing.T) {
	for recipient, want := range wantDonors {
		got, ok := CompatibleDonorTypes(recipient)
		if !ok {
			t.Fatalf("%s: expected ok=true", recipient)
		}
		gotS, wantS := sortedTypes(got), sortedTypes(want)
		if len(gotS) != len(wantS) {
			t.Fatalf("%s: got %v, want %v", recipient, got, want)
		}
		for i := range gotS {
			if gotS[i] != wantS[i] {
				t.Fatalf("%s: got %v, want %v", recipient, got, want)
			}
		}
	}
}

func TestCompatibleDonorTypes_Invalid(t *testing.T) {
	for _, bad := range []BloodType{BloodUnknown, "", "Z+", "a+"} {
		if _, ok := CompatibleDonorTypes(bad); ok {
			t.Errorf("CompatibleDonorTypes(%q): expected ok=false", bad)
		}
	}
}

func TestCompatibleDonorTypes_ReturnsCopy(t *testing.T) {
	got, _ := CompatibleDonorTypes(ABPos)
	got[0] = "tampered"
	again, _ := CompatibleDonorTypes(ABPos)
	if again[0] == "tampered" {
		t.Fatal("CompatibleDonorTypes returned a slice aliasing internal state")
	}
}

func TestUniversalDonorAndRecipient(t *testing.T) {
	// O- recipient accepts only O- donors.
	if donors, _ := CompatibleDonorTypes(ONeg); len(donors) != 1 || donors[0] != ONeg {
		t.Errorf("O- recipient: got %v, want [O-]", donors)
	}
	// AB+ recipient accepts all 8 donor types.
	if donors, _ := CompatibleDonorTypes(ABPos); len(donors) != 8 {
		t.Errorf("AB+ recipient: got %d donor types, want 8", len(donors))
	}
	// O- donor can give to every recipient.
	for recipient := range wantDonors {
		if !IsCompatible(ONeg, recipient) {
			t.Errorf("O- donor should be compatible with %s", recipient)
		}
	}
}

func TestIsCompatible_UnknownDonorNeverCompatible(t *testing.T) {
	for recipient := range wantDonors {
		if IsCompatible(BloodUnknown, recipient) {
			t.Errorf("unknown donor must not be compatible with %s", recipient)
		}
	}
}

func TestIsCompatible_Spot(t *testing.T) {
	cases := []struct {
		donor, recipient BloodType
		want             bool
	}{
		{APos, APos, true},
		{OPos, ANeg, false}, // O+ carries Rh+, A- recipient is Rh-
		{BNeg, ABPos, true},
		{ABPos, ABPos, true},
		{ABPos, OPos, false}, // AB+ donor only to AB+ recipient
	}
	for _, c := range cases {
		if got := IsCompatible(c.donor, c.recipient); got != c.want {
			t.Errorf("IsCompatible(%s→%s)=%v, want %v", c.donor, c.recipient, got, c.want)
		}
	}
}

func TestBloodTypeValidation(t *testing.T) {
	if !IsValidBloodType(OPos) || IsValidBloodType(BloodUnknown) {
		t.Error("IsValidBloodType: unknown must be invalid, O+ valid")
	}
	if !IsValidDonorBloodType(BloodUnknown) || !IsValidDonorBloodType(OPos) {
		t.Error("IsValidDonorBloodType: both unknown and O+ must be valid")
	}
	if IsValidDonorBloodType("nonsense") {
		t.Error("IsValidDonorBloodType: garbage must be invalid")
	}
}

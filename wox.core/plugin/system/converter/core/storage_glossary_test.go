package core

import "testing"

func TestResolveStorageUnit_ByteAliasesShareCanonicalUnit(t *testing.T) {
	glossary := NewStorageGlossary()

	aliases := []string{"b", "byte", "bytes"}
	for _, alias := range aliases {
		unit, ok := glossary.ResolveStorageUnit(alias)
		if !ok {
			t.Fatalf("expected alias %q to resolve", alias)
		}
		if unit.Symbol != "B" {
			t.Fatalf("expected alias %q to resolve to symbol B, got %q", alias, unit.Symbol)
		}
		if unit.Family != StorageUnitFamilyByteBase {
			t.Fatalf("expected alias %q to resolve to byte-base family, got %q", alias, unit.Family)
		}
	}
}

func TestResolveStorageUnit_GigabyteDisambiguationContract(t *testing.T) {
	glossary := NewStorageGlossary()

	gb, ok := glossary.ResolveStorageUnit("gb")
	if !ok {
		t.Fatalf("expected gb to resolve")
	}
	if gb.Symbol != "GB" || gb.Family != StorageUnitFamilyDecimal {
		t.Fatalf("expected gb to resolve to decimal GB, got symbol=%q family=%q", gb.Symbol, gb.Family)
	}

	gib, ok := glossary.ResolveStorageUnit("gib")
	if !ok {
		t.Fatalf("expected gib to resolve")
	}
	if gib.Symbol != "GiB" || gib.Family != StorageUnitFamilyBinary {
		t.Fatalf("expected gib to resolve to binary GiB, got symbol=%q family=%q", gib.Symbol, gib.Family)
	}
}

func TestResolveStorageUnit_SymbolAndFullWordFormsAreEquivalent(t *testing.T) {
	glossary := NewStorageGlossary()

	cases := []struct {
		symbol string
		word   string
	}{
		{symbol: "GB", word: "gigabyte"},
		{symbol: "MiB", word: "mebibyte"},
	}

	for _, tc := range cases {
		symbolUnit, ok := glossary.ResolveStorageUnit(tc.symbol)
		if !ok {
			t.Fatalf("expected symbol %q to resolve", tc.symbol)
		}
		wordUnit, ok := glossary.ResolveStorageUnit(tc.word)
		if !ok {
			t.Fatalf("expected full-word %q to resolve", tc.word)
		}

		if symbolUnit != wordUnit {
			t.Fatalf("expected %q and %q to resolve to same canonical unit, got %#v vs %#v", tc.symbol, tc.word, symbolUnit, wordUnit)
		}
	}
}

func TestResolveStorageUnit_NormalizationContract_TrimAndCaseInsensitive(t *testing.T) {
	glossary := NewStorageGlossary()

	cases := []struct {
		input  string
		symbol string
		family StorageUnitFamily
	}{
		{input: "  BYTES  ", symbol: "B", family: StorageUnitFamilyByteBase},
		{input: "\tGb\n", symbol: "GB", family: StorageUnitFamilyDecimal},
		{input: "  giBiByTeS ", symbol: "GiB", family: StorageUnitFamilyBinary},
	}

	for _, tc := range cases {
		unit, ok := glossary.ResolveStorageUnit(tc.input)
		if !ok {
			t.Fatalf("expected normalized input %q to resolve", tc.input)
		}
		if unit.Symbol != tc.symbol || unit.Family != tc.family {
			t.Fatalf("expected input %q to resolve to symbol=%q family=%q, got symbol=%q family=%q", tc.input, tc.symbol, tc.family, unit.Symbol, unit.Family)
		}
	}
}

func TestResolveStorageUnit_SupportedRangeBoundary_IsExplicit(t *testing.T) {
	glossary := NewStorageGlossary()

	supported := []string{"b", "kb", "mb", "gb", "tb", "kib", "mib", "gib", "tib"}
	for _, alias := range supported {
		if _, ok := glossary.ResolveStorageUnit(alias); !ok {
			t.Fatalf("expected supported alias %q to resolve", alias)
		}
	}

	unsupportedBeyondBoundary := []string{"pb", "pib", "petabyte", "pebibyte"}
	for _, alias := range unsupportedBeyondBoundary {
		if _, ok := glossary.ResolveStorageUnit(alias); ok {
			t.Fatalf("expected boundary alias %q to be unsupported", alias)
		}
	}
}

func TestResolveStorageUnit_Guardrails_UnsupportedOrAvoidTermsDoNotResolve(t *testing.T) {
	glossary := NewStorageGlossary()

	unsupported := []string{"", "bit", "bits", "kilobit", "kbit", "mbit", "byte/s"}
	for _, term := range unsupported {
		if _, ok := glossary.ResolveStorageUnit(term); ok {
			t.Fatalf("expected unsupported/avoid term %q to not resolve", term)
		}
	}
}

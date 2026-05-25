package core

import "strings"

type StorageUnitFamily string

const (
	StorageUnitFamilyByteBase StorageUnitFamily = "byte-base"
	StorageUnitFamilyDecimal  StorageUnitFamily = "decimal"
	StorageUnitFamilyBinary   StorageUnitFamily = "binary"
)

type StorageUnit struct {
	Symbol string
	Family StorageUnitFamily
}

type StorageGlossary struct {
	aliases map[string]StorageUnit
}

func NewStorageGlossary() StorageGlossary {
	aliases := map[string]StorageUnit{}

	// Canonical unit range is intentionally bounded to byte through tera for both
	// decimal (B, KB, MB, GB, TB) and binary (B, KiB, MiB, GiB, TiB) families.
	registerAliases(aliases, StorageUnit{Symbol: "B", Family: StorageUnitFamilyByteBase}, "b", "byte", "bytes")
	registerAliases(aliases, StorageUnit{Symbol: "KB", Family: StorageUnitFamilyDecimal}, "kb", "kilobyte", "kilobytes")
	registerAliases(aliases, StorageUnit{Symbol: "MB", Family: StorageUnitFamilyDecimal}, "mb", "megabyte", "megabytes")
	registerAliases(aliases, StorageUnit{Symbol: "GB", Family: StorageUnitFamilyDecimal}, "gb", "gigabyte", "gigabytes")
	registerAliases(aliases, StorageUnit{Symbol: "TB", Family: StorageUnitFamilyDecimal}, "tb", "terabyte", "terabytes")

	registerAliases(aliases, StorageUnit{Symbol: "KiB", Family: StorageUnitFamilyBinary}, "kib", "kibibyte", "kibibytes")
	registerAliases(aliases, StorageUnit{Symbol: "MiB", Family: StorageUnitFamilyBinary}, "mib", "mebibyte", "mebibytes")
	registerAliases(aliases, StorageUnit{Symbol: "GiB", Family: StorageUnitFamilyBinary}, "gib", "gibibyte", "gibibytes")
	registerAliases(aliases, StorageUnit{Symbol: "TiB", Family: StorageUnitFamilyBinary}, "tib", "tebibyte", "tebibytes")

	return StorageGlossary{aliases: aliases}
}

func (g StorageGlossary) ResolveStorageUnit(input string) (StorageUnit, bool) {
	unit, ok := g.aliases[normalizeStorageAlias(input)]
	return unit, ok
}

func normalizeStorageAlias(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func registerAliases(aliases map[string]StorageUnit, unit StorageUnit, names ...string) {
	for _, name := range names {
		aliases[normalizeStorageAlias(name)] = unit
	}
}

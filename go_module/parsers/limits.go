package parsers

// Engine buffer capacities and point-buy layout bounds. Keep consumers on
// these shared values so validation and UI reporting cannot drift.
const (
	MBCHMaxFileBytes    = 16384
	ClassInfoMaxBytes   = 8192
	WeaponInfoMaxBytes  = 4096
	ForceInfoMaxBytes   = 2048
	PairedValueMaxBytes = 2048
	KeyMaxBytes         = 256

	// Usable interior bytes (excluding outer braces but including nested, minus NUL)
	ClassInfoMaxPayload   = ClassInfoMaxBytes - 1
	WeaponInfoMaxPayload  = WeaponInfoMaxBytes - 1
	ForceInfoMaxPayload   = ForceInfoMaxBytes - 1
	PairedValueMaxPayload = PairedValueMaxBytes - 1
	KeyMaxPayload         = KeyMaxBytes - 1

	// Point-buy / custom-build layout capacities.
	PointbuyMaxArchetypes     = 3
	PointbuySlotsPerArchetype = 15
	PointbuyMaxTotalSlots     = PointbuyMaxArchetypes * PointbuySlotsPerArchetype
)

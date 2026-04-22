package main

import (
	_ "embed"
)

// Monospace (Hack) — code, source preview, anywhere TextStyle.Monospace.
//
//go:embed assets/font.ttf
var embedFont []byte

// Body / display (Jost) — bundled in three weights so theme.Font can
// return the right face for regular, semibold, and bold text styles.
// Jost is released under the SIL Open Font License v1.1 — see
// assets/JOST-OFL.txt. Replaced Futura (licensed, non-redistributable).

//go:embed assets/jost-regular.ttf
var embedJostRegular []byte

//go:embed assets/jost-semibold.ttf
var embedJostSemibold []byte

//go:embed assets/jost-bold.ttf
var embedJostBold []byte

// Panel-push icons — boxicons-style arrows+bars that communicate
// "this pane is about to slide off/onto the edge." Used by the
// sidebar header's collapse/expand toggle and by the source panel's
// matching toggle. These beat theme.NavigateBack/Next because the
// bar+arrow combo makes the destination unambiguous (the pane, not
// a generic "back").
//
//go:embed assets/panel-collapse-left.svg
var embedPanelCollapseLeft []byte

//go:embed assets/panel-expand-left.svg
var embedPanelExpandLeft []byte

//go:embed assets/panel-collapse-right.svg
var embedPanelCollapseRight []byte

//go:embed assets/panel-expand-right.svg
var embedPanelExpandRight []byte

// MBII logo, shown on the welcome screen's hero next to the word
// "FOUNDRY". Bundled so the home screen renders identically regardless
// of whether the user's Dev SVN copy is mounted or not.
//
//go:embed assets/logo-mbii.png
var embedLogoMBII []byte

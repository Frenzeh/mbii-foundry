package main

import "testing"

// Round-trip a small fragment of typical MBII portrait shader syntax
// so we don't regress on shader-name lookup the next time someone
// touches parseShaderFile. Two cases:
//   1. shader-name { stage { map <texture> } } — the common case.
//   2. shader-name { surfaceparm nopicmip stage { map ... } } — top-
//      level directive before the stage opens.
func TestParseShaderFile_BasicMap(t *testing.T) {
	body := `
models/players/t_yoda/mb2_icon_default
{
    nopicmip
    {
        map models/players/t_yoda/mb2_icon_default_tx
        rgbGen identity
    }
}

gfx/hud/some_icon
{
    {
        clampmap gfx/hud/some_icon
    }
}
`
	out := map[string]string{}
	parseShaderFile(body, out)

	if got, want := out["models/players/t_yoda/mb2_icon_default"],
		"models/players/t_yoda/mb2_icon_default_tx"; got != want {
		t.Errorf("primary map mismatch:\n  got:  %q\n  want: %q", got, want)
	}
	if got, want := out["gfx/hud/some_icon"], "gfx/hud/some_icon"; got != want {
		t.Errorf("clampmap mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestParseShaderFile_AnimMapPicksFirst(t *testing.T) {
	body := `
gfx/effects/blood
{
    {
        animMap 5 gfx/effects/blood_1 gfx/effects/blood_2 gfx/effects/blood_3
    }
}
`
	out := map[string]string{}
	parseShaderFile(body, out)
	if got, want := out["gfx/effects/blood"], "gfx/effects/blood_1"; got != want {
		t.Errorf("animMap should pick first frame: got %q want %q", got, want)
	}
}

func TestParseShaderFile_LightmapSkipped(t *testing.T) {
	body := `
maps/level1/floor
{
    {
        map $lightmap
    }
    {
        map textures/wood/floor1
    }
}
`
	out := map[string]string{}
	parseShaderFile(body, out)
	// $lightmap is skipped; the second stage's map should win.
	if got, want := out["maps/level1/floor"], "textures/wood/floor1"; got != want {
		t.Errorf("expected to skip $lightmap and pick the real texture: got %q want %q", got, want)
	}
}

func TestParseShaderFile_LineComments(t *testing.T) {
	body := `
// header comment
shaders/test_a // trailing
{
    // inside
    {
        map textures/test_a // path
    }
}
`
	out := map[string]string{}
	parseShaderFile(body, out)
	if got, want := out["shaders/test_a"], "textures/test_a"; got != want {
		t.Errorf("line comments dropped wrong tokens: got %q want %q", got, want)
	}
}

// Resolve must not block when the resolver hasn't been built. A
// previous version called build() inline on first lookup, scanning
// every .shader file under the VFS — synchronous on the UI thread,
// which froze the app for seconds on a full MBII install when
// opening the first MBCH. Guard against the regression: a fresh
// resolver with a nil VFS should return "" instantly without
// spinning up any work.
func TestResolve_NonBlockingWithoutBuild(t *testing.T) {
	sr := NewShaderResolver(nil)
	got := sr.Resolve("models/players/t_yoda/mb2_icon_default")
	if got != "" {
		t.Errorf("Resolve before build should return empty, got %q", got)
	}
}

// Pre-built resolvers should serve cached lookups immediately without
// spinning up any further file I/O. Fast-path coverage for the common
// case after the background prebuild has completed.
func TestResolve_AfterPrebuild(t *testing.T) {
	sr := NewShaderResolver(nil) // nil VFS — Prebuild flips built=true with empty map
	sr.Prebuild()
	if got := sr.Resolve("anything"); got != "" {
		t.Errorf("empty resolver should return empty for any name, got %q", got)
	}
}

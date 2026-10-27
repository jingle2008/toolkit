package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// The frame fills the terminal exactly — no line wider than it, no more
// lines than it has — at any width, so nothing soft-wraps or scrolls.
func TestFrame_FitsEveryWidth(t *testing.T) {
	t.Parallel()
	for _, cat := range []domain.Category{domain.DedicatedAICluster, domain.GPUNode} {
		m := newGoldenModel(t, 80, 24)
		showCategory(t, m, cat)
		for w := 30; w <= 200; w += 7 {
			m.updateLayout(w, 24)
			lines := strings.Split(ansi.Strip(m.render()), "\n")
			assert.Len(t, lines, 24, "%s width %d", cat, w)
			for i, l := range lines {
				assert.LessOrEqual(t, ansi.StringWidth(l), w, "%s width %d line %d: %q", cat, w, i, l)
			}
		}
	}
}

func TestFrame_Layout(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.DedicatedAICluster)
	lines := strings.Split(ansi.Strip(m.render()), "\n")
	assert.Contains(t, lines[0], "oc1 ›  PROD  › us-ashburn-1 · dp-prod-iad · k8s v1.31.4", "prod is a badge")
	assert.Contains(t, lines[0], "v1.2.3")
	assert.True(t, strings.HasPrefix(lines[1], " DedicatedAICluster "), lines[1])
	assert.True(t, strings.HasPrefix(lines[2], "╭────"), "the breadcrumb names the category; the title has nothing to add: %s", lines[2])
	assert.NotContains(t, lines[1], "[1/3]", "the position moved to the panel")
	assert.True(t, strings.HasSuffix(lines[len(lines)-2], " 1 of 3 ─╯"), "position on the bottom edge: %s", lines[len(lines)-2])
	assert.True(t, strings.HasSuffix(strings.TrimRight(lines[len(lines)-1], " "), "? more"), lines[len(lines)-1])
	assert.NotContains(t, strings.Join(lines, "\n"), "|_   _|", "no logo in the frame")
}

// The footer drops hints from the right to fit, never "? more".
func TestFooter_TrimsToFit(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 200, 30)
	showCategory(t, m, domain.DedicatedAICluster)
	wide := ansi.Strip(m.footer())
	assert.Contains(t, wide, "o go to related")
	assert.Contains(t, wide, "s sort")
	assert.Contains(t, wide, "^x delete")
	m.updateLayout(40, 30)
	narrow := ansi.Strip(m.footer())
	assert.LessOrEqual(t, lipgloss.Width(narrow), 40)
	assert.True(t, strings.HasSuffix(narrow, "? more"), narrow)
	assert.Less(t, len(narrow), len(wide))
}

func TestBreadcrumbChips(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 160, 30)
	showCategory(t, m, domain.DedicatedAICluster)
	assert.Empty(t, m.breadcrumbChips(), "nothing active, no chips")

	m.filter = "gpu"
	m.sortColumn, m.sortAsc = common.AgeCol, false
	m.showFaulty = true
	m.history = histOf(domain.Tenant, domain.DedicatedAICluster)
	m.historyIdx = 1
	chips := ansi.Strip(m.breadcrumbChips())
	for _, want := range []string{"⟨/gpu⟩", "⟨sort: age ▼⟩", "⟨faulty⟩", "⟨◀ 2/2⟩"} {
		assert.Contains(t, chips, want)
	}
	m.scope = &domain.Scope{Category: domain.Tenant, Name: "acme"}
	assert.Equal(t, "DedicatedAICluster › acme", m.contextString())
}

func TestEnvBar_QuickSwitchStrip(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	m.updateLayout(200, 40)
	bar := ansi.Strip(m.envBar())
	strip := ansi.Strip(m.quickSwitchStrip())
	require.NotEmpty(t, strip)
	assert.Contains(t, bar, strip, "other slots listed when they fit")
	assert.NotContains(t, strip, currentSlotMarker, "the current slot is the env bar itself")
	m.updateLayout(60, 40)
	assert.NotContains(t, ansi.Strip(m.envBar()), strip, "dropped when it doesn't fit")
}

// Every value a Status column produces has a pinned tone; anything else
// stays plain.
func TestToneOf(t *testing.T) {
	t.Parallel()
	for cell, want := range map[string]statusTone{
		// Tenant
		"ACTIVE": toneSuccess, "DELETING": toneDanger, "TERMINATED": toneDanger, "ALWAYS_FREE_ONLY": tonePlain,
		// DAC
		"Active": toneSuccess, "Failed": toneDanger, "Deleting": toneDanger, "pending": toneWarn,
		// BaseModel, ImportedModel
		"Ready": toneSuccess, "Creating": toneWarn, "Unknown": toneWarn,
		// GPU node
		"OK": toneSuccess, "WARN: CORDONED": toneWarn, "ERROR: Missing GPUs": toneDanger,
		"ERROR: Unhealthy": toneDanger, "ERROR: Not ready": toneDanger,
		// GPU pool
		"RUNNING": toneSuccess, "SCALING": toneWarn, "PROVISIONING": toneWarn, "STARTING": toneWarn,
		"STOPPING": toneWarn, "STOPPED": tonePlain, "TERMINATING": toneWarn, "FAILED": toneDanger,
		"UNKNOWN": toneWarn, "NONEXIST": toneDanger, "...": toneWarn,
		// ServingRuntime
		"live": toneSuccess, "off": tonePlain, "disabled": tonePlain,
		// Unlisted values stay plain, including look-alikes.
		"": tonePlain, "INACTIVE": tonePlain, "BROKEN": tonePlain, "Updating": tonePlain,
	} {
		assert.Equal(t, want, toneOf(cell), "%q", cell)
	}
}

// Status cells are tinted in the table but the raw rows stay plain, since
// filtering, copying and row identity read them.
func TestApplyRows_TonesStatusOnly(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 160, 30)
	showCategory(t, m, domain.DedicatedAICluster)
	view := m.table.View()
	assert.Contains(t, view, fgSeq(m.theme.Palette.Danger)+"Failed")
	assert.Contains(t, view, fgSeq(m.theme.Palette.Success)+"Active")
	for _, r := range m.rawRows {
		for _, c := range r {
			assert.NotContains(t, c, "\x1b[", "raw rows stay plain")
		}
	}
}

func TestEnvBar_ProductionBadgeOnlyForProd(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.DedicatedAICluster)
	for _, typ := range []string{"dev", "preprod"} {
		m.environment.Type = typ
		bar := ansi.Strip(m.envBar())
		assert.Contains(t, bar, "oc1 › "+typ+" › ", typ)
		assert.NotContains(t, bar, "PROD", typ)
	}
}

// With a filter or faulty-only mode hiding rows, the bottom edge gives
// the total too.
func TestPosition_ShowsTotalWhenRowsHidden(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.DedicatedAICluster)
	assert.Equal(t, "1 of 3", m.positionText())

	m.filter = "alpha"
	m.updateRows(true)
	assert.Equal(t, "1 of 1 (3 total)", m.positionText())

	m.filter = "zzz"
	m.updateRows(true)
	assert.Equal(t, "0 of 0 (3 total)", m.positionText())

	m.filter = ""
	m.showFaulty = true
	m.updateRows(true)
	assert.Equal(t, "1 of 1 (3 total)", m.positionText(), "faulty-only hides the healthy ones")

	_, cmd := m.Update(tableRowsComputedMsg{Rows: m.table.Rows(), Total: 9, Gen: m.gens.rows})
	assert.Nil(t, cmd)
	assert.Equal(t, "1 of 1 (9 total)", m.positionText(), "the async path carries its total")
}

// An empty list offers no row actions or sort, only its list-level
// toggles and the core keys.
func TestFooter_EmptyListDropsRowActions(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 200, 30)
	showCategory(t, m, domain.GPUNode)
	m.applyRows(nil, nil, false)
	footer := ansi.Strip(m.footer())
	for _, gone := range []string{"go to related", "drain", "s sort"} {
		assert.NotContains(t, footer, gone)
	}
	for _, kept := range []string{"^z faulty only", "r refresh", "/ filter", "? more"} {
		assert.Contains(t, footer, kept)
	}
}

// Under a modal, the view's footer is blanked: its keys don't work there.
func TestOverlayModal_BlanksFooter(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.GPUNode)
	assert.Contains(t, ansi.Strip(m.render()), "go to related")
	m.openPalette()
	out := ansi.Strip(m.render())
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 40)
	assert.Empty(t, strings.TrimSpace(lines[len(lines)-1]), "footer blanked under the modal")
	assert.NotContains(t, out, "go to related")
}

// The panel title names the hidden columns, and falls back to a count
// when the names don't fit.
func TestTableTitle_NamesHiddenColumns(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	assert.Empty(t, m.tableTitle())
	m.hiddenColumns = []string{"Internal", "Note"}
	assert.Equal(t, "Internal, Note hidden", m.tableTitle())
	m.viewWidth = 40
	assert.Equal(t, "2 columns hidden", m.tableTitle())
}

// A resize cuts middle-truncated cells (OCIDs) to the new widths, keeping
// their distinguishing tail, rather than leaving the table to cut their
// end.
func TestUpdateLayout_RetruncatesInTheMiddle(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.Tenant)
	col := slices.IndexFunc(m.headers, func(h header) bool { return h.text == "OCID" })
	require.GreaterOrEqual(t, col, 0)
	m.updateLayout(50, 30)
	w := m.table.Columns()[col].Width
	require.Positive(t, w)
	cell := ansi.Strip(m.table.Rows()[0][col]) // OCIDs are drawn Muted
	assert.Contains(t, cell, "…")
	assert.True(t, strings.HasSuffix(cell, "acme"), "the tail survives: %q", cell)
	assert.LessOrEqual(t, ansi.StringWidth(cell), w)
}

// The history chip has an arrow for each way [ and ] can still go.
func TestHistoryChip_ArrowsFollowDirections(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	m.history = histOf(domain.Tenant, domain.GPUNode, domain.GPUPool)
	for idx, want := range []string{"⟨1/3 ▶⟩", "⟨◀ 2/3 ▶⟩", "⟨◀ 3/3⟩"} {
		m.historyIdx = idx
		assert.Equal(t, want, m.historyChip())
	}
}

// While typing a filter the breadcrumb reads "/text", like the ⟨/text⟩
// chip the filter becomes once applied.
func TestStatusView_FilterPromptIsSlash(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.Tenant)
	m.enterFilterMode()
	m.textInput.SetValue("acme")
	assert.Contains(t, ansi.Strip(m.statusView()), "Tenant   /acme", "aligned with the applied chip: Tenant   ⟨/acme⟩")
}

// On a narrow terminal the stats give way before the filter chip is cut;
// with room for both, both show.
func TestStatusView_StatsGiveWayToChips(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 40, 20)
	showCategory(t, m, domain.Tenant)
	m.stats = tableStats{"Free": 15, "Total": 144}
	m.filter = "h100_cn"

	line := ansi.Strip(m.statusView())
	assert.Contains(t, line, "⟨/h100_cn⟩", "the filter chip is whole")
	assert.NotContains(t, line, "Free: 15", "the stats gave way")
	assert.LessOrEqual(t, ansi.StringWidth(line), 40)

	m.viewWidth = 120
	line = ansi.Strip(m.statusView())
	assert.Contains(t, line, "⟨/h100_cn⟩")
	assert.Contains(t, line, "Free: 15 • Total: 144")
}

// The focused panel's border is Accent in every full-screen view: the
// list, details and the log.
func TestFrame_FocusedPanelBorderIsAccent(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	accent := strings.TrimSuffix(strings.TrimPrefix(fgSeq(m.theme.Palette.Accent), "\x1b["), "m") // 38;2;r;g;b
	topEdge := func() string { return strings.Split(m.render(), "\n")[2] }

	showCategory(t, m, domain.GPUNode)
	assert.Contains(t, topEdge(), accent, "list")
	showDetails(t, m)
	assert.Contains(t, topEdge(), accent, "details")
	_ = m.openLog()
	assert.Contains(t, topEdge(), accent, "log")
}

// Number columns are right-aligned, header included; OCIDs are Muted.
func TestDecorateRows_AlignsNumbersAndMutesIDs(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.GPUNode)
	free := slices.IndexFunc(m.headers, func(h header) bool { return h.text == "Free" })
	require.GreaterOrEqual(t, free, 0)
	w := m.table.Columns()[free].Width
	cell := ansi.Strip(m.table.Rows()[0][free])
	assert.Len(t, cell, w)
	assert.True(t, strings.HasSuffix(cell, "2") && strings.HasPrefix(cell, " "), "right-aligned: %q", cell)
	title := ansi.Strip(m.table.Columns()[free].Title)
	assert.True(t, strings.HasSuffix(title, "Free") && strings.HasPrefix(title, " "), "header right-aligned: %q", title)

	showCategory(t, m, domain.Tenant)
	ocid := slices.IndexFunc(m.headers, func(h header) bool { return h.text == "OCID" })
	require.GreaterOrEqual(t, ocid, 0)
	assert.True(t, strings.HasPrefix(m.table.Rows()[0][ocid], fgSeq(m.theme.Palette.Muted)), "OCID in Muted")
}

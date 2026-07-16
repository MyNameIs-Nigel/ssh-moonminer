package tui

import (
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func (g *Game) keyMining(m tea.KeyPressMsg) []tea.Cmd {
	k := m.String()
	run := g.snap.State.Run
	if run == nil {
		return nil
	}
	switch run.Phase {
	case sim.PhaseMining:
		switch k {
		case "b", "esc", "enter":
			snap, err := g.sess.BailOrDepart(g.now)
			return g.refreshSnap(snap, err)
		}
		if isSkillCheckKey(k) {
			if run.SkillCheck == nil {
				return nil
			}
			snap, err := g.sess.AttemptSkillCheck(g.now)
			return g.refreshSnap(snap, err)
		}
	case sim.PhaseTribute:
		switch k {
		case "d":
			snap, err := g.sess.AcceptTribute(g.now)
			return g.refreshSnap(snap, err)
		case "r", "esc":
			snap, err := g.sess.RefuseTribute(g.now)
			return g.refreshSnap(snap, err)
		case "f":
			if !sim.HasWeapon(&g.snap.State) {
				return nil
			}
			snap, err := g.sess.FightPirates(g.now)
			return g.refreshSnap(snap, err)
		}
	case sim.PhaseEscaping:
		// No menu actions are accepted while fleeing — the ship either
		// escapes or dies.
	case sim.PhaseCombat:
		return g.keyCombat(m)
	}
	return nil
}

// Bubble Tea identifies physical space-bar presses as "space". Keep the
// literal-space form too because the shipyard mouse hitbox synthesizes it.
func isSkillCheckKey(k string) bool { return k == "space" || k == " " }

func (g *Game) renderMining() string {
	st := g.snap.State
	run := st.Run
	if run == nil {
		return g.renderBelt()
	}
	ast, _ := sim.FindAsteroid(&st, run.AsteroidID)
	name := "???"
	tier := 0
	value := 0
	if ast != nil {
		name, tier, value = ast.Name, ast.Tier, ast.Value
	}

	switch run.Phase {
	case sim.PhaseTribute:
		return g.renderTribute(&st, run, name, tier, value)
	case sim.PhaseEscaping:
		return g.renderEscape(&st, run, name, tier)
	case sim.PhaseCombat:
		return g.renderCombat(&st, run)
	default:
		return g.renderDrilling(&st, run, ast, name, tier, value)
	}
}

func (g *Game) renderEventBadge(st *sim.State, run *sim.ActiveRun) string {
	if run.ActiveEvent == nil {
		return ""
	}
	switch run.ActiveEvent.Kind {
	case sim.EventPowerOutage:
		return theme.DimStyle.Render("░░ POWER OUTAGE — SYSTEMS DARK ░░")
	case sim.EventRadarBlackout:
		return theme.Red.Render("▓▓ RADAR BLACKOUT — SIGNAL LOST ▓▓")
	case sim.EventLifeSupportFailure:
		if run.LifeSupportBreached {
			return theme.Red.Render("‼ LIFE SUPPORT FAILURE — LEAVE NOW ‼")
		}
		return theme.Amber.Render(fmt.Sprintf("⚠ LIFE SUPPORT ALARM — %.0fs", run.ActiveEvent.Remaining))
	case sim.EventCargoShift:
		return theme.Amber.Render("▦ CARGO SHIFT — HOLD UNSTABLE, ESCAPE SLOWER")
	case sim.EventReactorSurge:
		return theme.Amber.Render("☀ REACTOR SURGE — FUEL BURN SPIKING")
	}
	return ""
}

func (g *Game) renderDrilling(st *sim.State, run *sim.ActiveRun, ast *sim.Asteroid, name string, tier, value int) string {
	// Resource left is always the asteroid's actual remaining ore. Filling a
	// cargo hold stops the drill, but it must never make the rock look empty.
	resourcePct := 0.0
	if ast != nil && ast.Volume > 0 {
		resourcePct = clampF(100*(1-sim.RunExtractedUnits(run)/float64(ast.Volume)), 0, 100)
	}
	depleted := sim.RunDepleted(st, g.content, ast, run)
	cargoFull := sim.RunCargoFull(st, g.content, run)

	title := fmt.Sprintf("%s MINING SITE — %s (%s %s)", theme.Glyph("diamond", st.Settings.ASCIISafe),
		name, g.content.Tiers.Glyphs[tier], g.content.Tiers.Labels[tier])
	title = theme.Amber.Render(title)

	fuelAmount := sim.FuelAmount(st, g.content)
	fuelPct := fuelAmount / sim.TankSize(st, g.content) * 100
	hullPct := sim.HullPct(st, g.content)

	leftW, rightW := g.miningColumnWidths()
	leftLines := []string{title}
	if sim.FuelMinerRecoveryMul(st, g.content) > 0 {
		if run.FuelAsteroid {
			leftLines = append(leftLines, theme.Green.Render("FUEL SCAN: RICH VEIN — FUEL MINER RECOVERING DRILL BURN"))
		} else {
			leftLines = append(leftLines, theme.DimStyle.Render("FUEL SCAN: NO FUEL VEIN DETECTED"))
		}
	}
	if badge := g.renderEventBadge(st, run); badge != "" {
		leftLines = append(leftLines, badge)
	}
	if run.EMPActive {
		leftLines = append(leftLines, theme.Cyan.Render(fmt.Sprintf("EMP LAUNCHER DEPLOYED — PIRATES STALLED %.0fs", run.EMPRemaining)))
	}
	leftLines = append(leftLines,
		fmt.Sprintf("%s RESOURCE LEFT %s %s",
			theme.Glyph("drill", st.Settings.ASCIISafe),
			theme.DrillBar(resourcePct, 26),
			theme.TxtStyle.Render(fmt.Sprintf("%3.0f%%", resourcePct))),
		g.renderHullLine(st, hullPct, 26),
		fmt.Sprintf("%s FUEL         %s %s",
			theme.Glyph("fuel", st.Settings.ASCIISafe),
			theme.FuelBar(fuelPct, 26),
			theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%.0f/%.0f %3.0f%%", fuelAmount, sim.TankSize(st, g.content), fuelPct))),
		fmt.Sprintf("CURRENT CUT VALUE  %s %s of %s %s (not sold)",
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.Gold.Render(fmt.Sprintf("%d", run.CargoValue)),
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.TxtStyle.Render(fmt.Sprintf("%d", value))),
	)

	// Keep the countdown in the left status column. The pressure point itself
	// still flashes on the asteroid, but its timer must remain readable without
	// crossing the cockpit to the right-hand scanner/asteroid viewport.
	skillRow := -1
	if sc := run.SkillCheck; sc != nil {
		remainingPct := clampF(100*(1-sc.Elapsed/sc.Window), 0, 100)
		remainingSecs := math.Max(0, sc.Window-sc.Elapsed)
		skillRow = len(leftLines)
		label := "[SPACE] FRACTURE"
		if !st.Settings.ReducedMotion && g.tickCount%2 == 0 {
			label = theme.Gold.Bold(true).Render(label)
		} else {
			label = theme.Gold.Render(label)
		}
		leftLines = append(leftLines,
			theme.Gold.Render("⚙ PRESSURE POINT ACTIVE"),
			fmt.Sprintf("  %s %s %s", theme.RampBar(remainingPct, 16, false),
				theme.GaugeStyle(remainingPct, false).Render(fmt.Sprintf("%.1fs", remainingSecs)), label),
		)
	}

	var actionLine string
	if depleted {
		actionLine = theme.Button("ENTER", "DEPART", "", true, theme.HueGreen)
	} else if cargoFull {
		actionLine = theme.Amber.Render("HOLD FULL — [B] BAIL WITH CURRENT LOAD")
	} else if st.Settings.ReducedMotion {
		actionLine = theme.Amber.Render("[B] BAIL !")
	} else if g.tickCount%2 == 0 {
		actionLine = theme.Amber.Render("[B] BAIL")
	} else {
		actionLine = theme.Gold.Render("[B] BAIL")
	}
	actionRow := len(leftLines)
	leftLines = append(leftLines, actionLine)

	bodyH := g.height - chromeH - 2
	leftLines = fitMiningColumn(leftLines, leftW)
	for len(leftLines) < bodyH {
		leftLines = append(leftLines, "")
	}
	if skillRow >= 0 {
		g.addHit(g.contentX(), bodyLineY(skillRow), leftW, 2, "btn:skillcheck", nil)
	}
	g.hitBodyLine(actionRow, 1, actionLine, "btn:bail", nil)

	viewport, activePoint := g.renderAsteroidViewport(st, run, rightW, bodyH)
	if activePoint {
		// The right-hand cockpit viewport — the area bounded by the four blue
		// corners — is the mouse target for the currently lit surface point.
		g.addHit(g.contentX()+leftW+1, bodyLineY(0), rightW, bodyH, "btn:skillcheck", nil)
	}

	hint := "B BAIL"
	if depleted {
		hint = "ENTER DEPART"
	}
	if cargoFull {
		hint = "HOLD FULL — B BAIL"
	}
	if run.SkillCheck != nil {
		hint = "SPACE FRACTURE PRESSURE POINT · " + hint
	}
	if fuelPct <= 0 {
		hint = theme.Red.Render("▼ TANKS DRY — DRILL PAUSED, DECIDE NOW")
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		strings.Join(leftLines, "\n"), " ", viewport)
	return g.renderBottomKeybar(body, hint)
}

func (g *Game) miningColumnWidths() (left, right int) {
	// Reserve a 33-column viewport at the 80-column minimum: it fits the
	// 30-column asteroid art while leaving a stable, readable status column.
	left = min(50, max(46, g.contentWidth()-34))
	right = g.contentWidth() - left - 1
	return left, right
}

func fitMiningColumn(lines []string, width int) []string {
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "")
	}
	return lines
}

var asteroidSprites = [][]string{
	{
		"          .-~~~~~-.          ",
		"       .-'         '-.       ",
		"     .'   .--. .--.   '.     ",
		"    /    /   V V   \\    \\    ",
		"   |     \\    ^    /     |   ",
		"    \\      '---'       /    ",
		"     '.             .'     ",
		"       '-._________.-'       ",
	},
	{
		"         .-''''''-.         ",
		"      .-'  _.._     '-.      ",
		"    .'   .'    '.     '.    ",
		"   /    /  .--.  \\      \\   ",
		"  |    |  (____)  |      |  ",
		"   \\    \\   __  /      /   ",
		"    '.   '.__..__.'    .'    ",
		"      '-.__________.-'      ",
	},
	{
		"          .-~~~~-.          ",
		"      _.-'   _    '-._      ",
		"   .-'   .-'  '-.     '-.   ",
		"  /    .'  .--.  '.     \\  ",
		" |    |   (____)   |     | ",
		"  \\    '.   __  .'     /  ",
		"   '-.    '-....-'   .-'   ",
		"      '-._          _.-'      ",
	},
}

var pressurePointAnchors = [8]struct{ row, col int }{
	{0, 15}, {1, 23}, {3, 27}, {6, 22}, {7, 10}, {4, 4}, {2, 7}, {3, 15},
}

// renderAsteroidField remains a test-friendly full-width wrapper around the
// cockpit viewport used by the real mining layout.
func (g *Game) renderAsteroidField(st *sim.State, run *sim.ActiveRun, height int) (string, bool) {
	return g.renderAsteroidViewport(st, run, g.contentWidth(), height)
}

// renderAsteroidViewport places the scanner and randomly selected ASCII
// asteroid inside the large right-hand four-corner cockpit area. Only the
// corners are drawn: the space between them stays open for the asteroid.
func (g *Game) renderAsteroidViewport(st *sim.State, run *sim.ActiveRun, width, height int) (string, bool) {
	if width < 4 || height < 3 {
		return "", false
	}
	innerW := width - 2
	border := theme.Cyan
	lines := make([]string, height)
	lines[0] = border.Render("╭──") + strings.Repeat(" ", max(0, width-6)) + border.Render("──╮")
	lines[height-1] = border.Render("╰──") + strings.Repeat(" ", max(0, width-6)) + border.Render("──╯")
	if height > 3 {
		lines[1] = border.Render("│") + strings.Repeat(" ", innerW) + border.Render("│")
		lines[height-2] = border.Render("│") + strings.Repeat(" ", innerW) + border.Render("│")
	}
	for i := 1; i < height-1; i++ {
		if lines[i] == "" {
			lines[i] = strings.Repeat(" ", width)
		}
	}

	// Restore the scanner to the upper part of the right-hand viewport — the
	// visual position called out in the original layout — rather than turning
	// it into a left-column status line.
	for row, scannerLine := range strings.Split(g.renderPirateRadar(run), "\n") {
		if row+1 >= height-2 {
			break
		}
		lines[row+1] = placeMiningViewportLine(scannerLine, width, 2, border)
	}

	sprite := asteroidSprites[run.AsteroidSprite%len(asteroidSprites)]
	pointAt := make(map[[2]int]sim.PressurePointStatus, len(run.PressurePoints))
	for _, point := range run.PressurePoints {
		anchor := pressurePointAnchors[point.Position%len(pressurePointAnchors)]
		pointAt[[2]int{anchor.row, anchor.col}] = point.Status
	}
	active := run.SkillCheck != nil
	scannerH := len(strings.Split(g.renderPirateRadar(run), "\n"))
	artStart := scannerH + 2
	if available := height - 1 - artStart - len(sprite); available > 0 {
		artStart += available / 2
	}
	for artRow, raw := range sprite {
		row := artStart + artRow
		if row >= height-2 {
			break
		}
		art := g.renderAsteroidArt(raw, artRow, pointAt, st.Settings.ReducedMotion)
		lines[row] = placeMiningViewportLine(art, width, max(0, (innerW-visibleWidth(art))/2), border)
	}
	return strings.Join(lines, "\n"), active
}

func placeMiningViewportLine(content string, width, desiredPad int, border lipgloss.Style) string {
	innerW := width - 2
	pad := min(max(0, desiredPad), max(0, innerW-visibleWidth(content)))
	body := strings.Repeat(" ", pad) + content
	body += strings.Repeat(" ", max(0, innerW-visibleWidth(body)))
	return border.Render("│") + body + border.Render("│")
}

func (g *Game) renderAsteroidArt(line string, row int, points map[[2]int]sim.PressurePointStatus, reducedMotion bool) string {
	var out strings.Builder
	for col, ch := range []rune(line) {
		status, hasPoint := points[[2]int{row, col}]
		if !hasPoint {
			out.WriteString(theme.DimStyle.Render(string(ch)))
			continue
		}
		switch status {
		case sim.PressurePointActive:
			if !reducedMotion && g.tickCount%2 == 0 {
				out.WriteString(theme.White.Bold(true).Render("*"))
			} else {
				out.WriteString(theme.Gold.Bold(true).Render("*"))
			}
		case sim.PressurePointHit:
			out.WriteString(theme.Green.Bold(true).Render("+"))
		case sim.PressurePointMissed:
			out.WriteString(theme.Red.Bold(true).Render("x"))
		default:
			out.WriteString(theme.Violet.Render("o"))
		}
	}
	return out.String()
}

func visibleWidth(s string) int {
	// The artwork itself is ASCII; stripping its ANSI paint is therefore a
	// simple and allocation-light way to preserve exact cockpit centering.
	return len([]rune(ansi.Strip(s)))
}

const pirateRadarW = 16
const pirateRadarH = 6

// renderPirateRadar draws the bottom-right mining-screen radar scope: a
// fixed player anchor and a red pirate blip that approaches from a
// cosmetic, per-run fixed bearing as PirateDistance closes. The fuzzed ETA
// range above it is the only arrival-time information ever shown — the
// true PirateDistance/rate never appear as an exact number.
func (g *Game) renderPirateRadar(run *sim.ActiveRun) string {
	blackout := run.ActiveEvent != nil && run.ActiveEvent.Kind == sim.EventRadarBlackout
	etaText := fmt.Sprintf("PIRATE ETA ~%.0f-%.0fs", run.PirateETAMin, run.PirateETAMax)
	if run.JammerRemaining > 0 {
		etaText = fmt.Sprintf("JAMMED %.0fs", run.JammerRemaining)
	} else if blackout {
		etaText = "CONTACT LOST — NO ETA"
	}
	if run.EMPActive {
		etaText = fmt.Sprintf("EMP DELAY ~%.0fs", run.EMPRemaining)
	}
	etaLine := theme.GaugeStyle(100-run.PirateDistance, true).Render(etaText)

	w, h := pirateRadarW, pirateRadarH
	anchorCol, anchorRow := w/2, h-1
	pirateCol := clampInt(int(run.PirateBearing*float64(w-1)), 0, w-1)
	distPct := clampF(run.PirateDistance, 0, 100) / 100
	pirateRow := clampInt(int((1-distPct)*float64(h-2)), 0, h-2)

	lines := make([]string, 0, h+1)
	lines = append(lines, etaLine)
	for r := 0; r < h; r++ {
		cells := make([]string, w)
		for c := 0; c < w; c++ {
			cells[c] = theme.DimStyle.Render("·")
		}
		if r == pirateRow {
			cells[pirateCol] = theme.Red.Render("●")
		}
		if r == anchorRow {
			cells[anchorCol] = theme.Cyan.Render("▲")
		}
		lines = append(lines, strings.Join(cells, ""))
	}
	return strings.Join(lines, "\n")
}

func (g *Game) renderTribute(st *sim.State, run *sim.ActiveRun, name string, tier, value int) string {
	pirateName := "PIRATE"
	if p := g.content.PirateByID(run.PirateID); p != nil {
		pirateName = p.Name
	}
	title := theme.Red.Render(theme.Glyph("skull", st.Settings.ASCIISafe) + " " + pirateName + " TRANSMISSION")
	totalCargo := st.CargoValue + run.CargoValue
	demand := int(float64(totalCargo) * g.content.Mining.TributeDemandPct)
	if demand > totalCargo {
		demand = totalCargo
	}
	remaining := g.content.Mining.TributeDecisionSeconds - run.TributeSecondsElapsed
	if remaining < 0 {
		remaining = 0
	}
	armed := sim.HasWeapon(st)
	lines := []string{
		title,
		"",
		theme.TxtStyle.Render(fmt.Sprintf(`"Drop %s %d from all cargo aboard or we open your hull."`,
			theme.Glyph("credit", st.Settings.ASCIISafe), demand)),
		"",
		fmt.Sprintf("ASTEROID  %s (%s %s)", theme.Gold.Render(name), g.content.Tiers.Glyphs[tier], g.content.Tiers.Labels[tier]),
		fmt.Sprintf("CARGO ABOARD  %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), totalCargo),
		fmt.Sprintf("RETAIN AFTER DROP  %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), totalCargo-demand),
	}
	if p := g.content.PirateByID(run.PirateID); p != nil {
		odds := int(math.Round(sim.EstimateOddsForRun(st, g.content, run) * 100))
		bountyLine := fmt.Sprintf("BOUNTY %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), p.Bounty)
		if armed {
			bountyLine += fmt.Sprintf("   EST. ODDS %d%%", odds)
		}
		lines = append(lines, theme.Amber.Render(bountyLine))
	}
	lines = append(lines,
		theme.DimStyle.Render(fmt.Sprintf("decision timeout: %.0fs", remaining)),
		"",
	)
	dropBtn := theme.Button("D", "DROP CARGO", "", true, theme.HueAmber)
	refuseBtn := theme.Button("R", "REFUSE / RUN", "", true, theme.HueRed)
	lines = append(lines, dropBtn, refuseBtn)
	g.hitBodyLine(len(lines)-2, 1, dropBtn, "btn:tribute:accept", nil)
	g.hitBodyLine(len(lines)-1, 1, refuseBtn, "btn:tribute:refuse", nil)
	hint := "D DROP CARGO · R REFUSE / RUN"
	if armed {
		odds := int(math.Round(sim.EstimateOddsForRun(st, g.content, run) * 100))
		fightBtn := theme.Button("F", fmt.Sprintf("FIGHT — EST. ODDS %d%%", odds), "", true, theme.HueRed)
		lines = append(lines, fightBtn)
		g.hitBodyLine(len(lines)-1, 1, fightBtn, "btn:tribute:fight", nil)
		hint = "D DROP CARGO · R REFUSE / RUN · F FIGHT"
	}
	return g.renderBottomKeybar(strings.Join(lines, "\n"), hint)
}

func (g *Game) renderEscape(st *sim.State, run *sim.ActiveRun, name string, tier int) string {
	title := fmt.Sprintf("%s ESCAPE BURN — %s", theme.Glyph("diamond", st.Settings.ASCIISafe), name)
	if run.UnderAttack {
		title = theme.Glyph("skull", st.Settings.ASCIISafe) + " UNDER FIRE — " + name
		if !st.Settings.ReducedMotion && g.tickCount%2 == 0 {
			title = theme.Red.Bold(true).Render(title)
		} else {
			title = theme.Red.Render(title)
		}
	} else {
		title = theme.Cyan.Render(title)
	}

	pct := 0.0
	if run.EscapeSecondsRequired > 0 {
		pct = clampF(run.EscapeSecondsElapsed/run.EscapeSecondsRequired*100, 0, 100)
	}
	remaining := run.EscapeSecondsRequired - run.EscapeSecondsElapsed
	if remaining < 0 {
		remaining = 0
	}
	fuelAmount := sim.FuelAmount(st, g.content)
	fuelPct := fuelAmount / sim.TankSize(st, g.content) * 100
	hullPct := sim.HullPct(st, g.content)

	lines := []string{title}
	if badge := g.renderEventBadge(st, run); badge != "" {
		lines = append(lines, badge)
	}
	lines = append(lines,
		"",
		fmt.Sprintf("%s ESCAPE VECTOR %s %s (%.0fs left)",
			theme.Glyph("drill", st.Settings.ASCIISafe),
			theme.DrillBar(pct, 26),
			theme.DrillStyle(pct).Render(fmt.Sprintf("%3.0f%%", pct)), remaining),
		g.renderHullLine(st, hullPct, 26),
		fmt.Sprintf("%s FUEL          %s %s",
			theme.Glyph("fuel", st.Settings.ASCIISafe),
			theme.FuelBar(fuelPct, 26),
			theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%.0f/%.0f %3.0f%%", fuelAmount, sim.TankSize(st, g.content), fuelPct))),
		"",
		fmt.Sprintf("CARGO VALUE IN HOLD  %s %s (not sold)",
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.Gold.Render(fmt.Sprintf("%d", run.CargoValue))),
		"",
		theme.DimStyle.Render("Ship is fleeing — no menu actions accepted."),
	)
	hint := "ESCAPING — NO INPUT ACCEPTED"
	if run.UnderAttack {
		hint = theme.Red.Render("UNDER FIRE — HULL FALLING")
	}
	return g.renderBottomKeybar(strings.Join(lines, "\n"), hint)
}

func (g *Game) renderHullLine(st *sim.State, hullPct float64, width int) string {
	hullBar := theme.HullBar(hullPct, width)
	shieldHP, shieldMax, shieldDamaged := sim.ShieldStatus(st, g.content)
	shieldLabel := ""
	if shieldMax > 0 {
		shieldPct := shieldHP / shieldMax * 100
		hullBar = theme.HullShieldBar(hullPct, shieldPct, width)
		shieldLabel = theme.Cyan.Render(fmt.Sprintf(" SHIELD %.0f/%.0f", shieldHP, shieldMax))
		if shieldDamaged {
			shieldLabel += " " + theme.Red.Render("DAMAGED")
		}
	}
	return fmt.Sprintf("%s HULL         %s %s%s",
		theme.Glyph("skull", st.Settings.ASCIISafe), hullBar,
		theme.HullStyle(hullPct).Render(fmt.Sprintf("%3.0f%%", hullPct)), shieldLabel)
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

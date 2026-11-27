package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Orange-accented palette preserving goauthorllm's warm signature identity.
const (
	colorOrange      = "#F97316" // primary vibrant orange accent
	colorOrangeDark  = "#C2410C" // deep orange for active tabs & primary buttons
	colorOrangeDeep  = "#7C2D12" // subtle dark orange tint
	colorOrangeLight = "#FFEDD5" // warm cream text on orange backgrounds
	colorOrangeSoft  = "#FDBA74" // soft orange highlight

	colorText        = "#F8FAFC" // primary crisp foreground
	colorBody        = "#E2E8F0" // editor text foreground
	colorMuted       = "#94A3B8" // secondary labels
	colorDim         = "#64748B" // subtle hints
	colorBorder      = "#334155" // inactive border
	colorBorderHover = "#FB923C" // hover border (warm light orange)
	colorSurface     = "#1E293B" // inactive pill / card surface
	colorSurfaceDeep = "#0F172A" // deep background tint

	colorGreen     = "#22C55E"
	colorGreenDark = "#14532D"
	colorGreenSoft = "#DCFCE7"

	colorRed     = "#EF4444"
	colorRedDark = "#7F1D1D"
	colorRedSoft = "#FEE2E2"

	colorAmber     = "#F59E0B"
	colorAmberDark = "#78350F"
	colorAmberSoft = "#FEF3C7"
)

var (
	styleBrand = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorOrange))

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorText))

	styleMuted = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted))

	styleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim))

	styleDiffDel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorRed)).
			Background(lipgloss.Color("#2A1215"))

	styleDiffIns = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorGreen)).
			Background(lipgloss.Color("#0F291A"))

	styleDiffCtx = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBody))
)

func renderBadge(label, bg, fg string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg)).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

func renderTab(label string, active, hovered bool) string {
	s := lipgloss.NewStyle().Padding(0, 1)
	switch {
	case active:
		s = s.Foreground(lipgloss.Color(colorText)).Background(lipgloss.Color(colorOrangeDark)).Bold(true)
	case hovered:
		s = s.Foreground(lipgloss.Color(colorOrangeLight)).Background(lipgloss.Color(colorOrangeDeep)).Bold(true)
	default:
		s = s.Foreground(lipgloss.Color(colorMuted)).Background(lipgloss.Color(colorSurface))
	}
	return s.Render(label)
}

func renderPill(label string, active, hovered bool) string {
	s := lipgloss.NewStyle().Padding(0, 1)
	switch {
	case active:
		s = s.Foreground(lipgloss.Color(colorOrangeLight)).Background(lipgloss.Color(colorOrangeDark)).Bold(true)
	case hovered:
		s = s.Foreground(lipgloss.Color(colorText)).Background(lipgloss.Color("#334155")).Underline(true)
	default:
		s = s.Foreground(lipgloss.Color(colorMuted)).Background(lipgloss.Color(colorSurface))
	}
	return s.Render(label)
}

func renderPrimaryBtn(label string, hovered bool) string {
	bg := colorOrangeDark
	if hovered {
		bg = colorOrange
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorText)).
		Background(lipgloss.Color(bg)).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

func renderDangerBtn(label string, hovered bool) string {
	bg := colorRedDark
	if hovered {
		bg = colorRed
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorRedSoft)).
		Background(lipgloss.Color(bg)).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

func renderSuccessBtn(label string, hovered bool) string {
	bg := colorGreenDark
	if hovered {
		bg = "#15803D"
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorGreenSoft)).
		Background(lipgloss.Color(bg)).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

func renderSecondaryBtn(label string, hovered bool) string {
	fg := colorBody
	bg := colorSurface
	if hovered {
		fg = colorOrangeLight
		bg = colorOrangeDeep
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg)).
		Bold(hovered).
		Padding(0, 1).
		Render(label)
}

func renderBox(title, rightMeta, body string, width int, focused, hovered bool) string {
	borderColor := lipgloss.Color(colorBorder)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorMuted))
	if hovered {
		borderColor = lipgloss.Color(colorBorderHover)
		titleStyle = titleStyle.Foreground(lipgloss.Color(colorOrangeSoft))
	}
	if focused {
		borderColor = lipgloss.Color(colorOrange)
		titleStyle = titleStyle.Foreground(lipgloss.Color(colorText))
	}

	innerWidth := max(8, width-4)
	headerLeft := titleStyle.Render(title)
	header := headerLeft
	if rightMeta != "" {
		metaRendered := styleDim.Render(rightMeta)
		gap := innerWidth - lipgloss.Width(headerLeft) - lipgloss.Width(metaRendered)
		if gap >= 2 {
			header = headerLeft + strings.Repeat(" ", gap) + metaRendered
		}
	}

	content := header + "\n" + body
	return lipgloss.NewStyle().
		Width(max(10, width-2)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(content)
}

func statusColor(level string) lipgloss.Color {
	switch level {
	case "success":
		return lipgloss.Color(colorGreen)
	case "error":
		return lipgloss.Color(colorRed)
	case "warn":
		return lipgloss.Color(colorAmber)
	case "accent":
		return lipgloss.Color(colorOrangeSoft)
	default:
		return lipgloss.Color(colorMuted)
	}
}

func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	return ansi.Truncate(s, maxWidth, "…")
}

func formatSpeedLabel(cps int) string {
	if cps <= 0 {
		return "Max"
	}
	return fmt.Sprintf("%d c/s", cps)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

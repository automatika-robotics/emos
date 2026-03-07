package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	ThemeRed     = lipgloss.Color("#d54e53")
	ThemeBlue    = lipgloss.Color("#81a2be")
	ThemeGreen   = lipgloss.Color("#b5bd68")
	ThemeYellow  = lipgloss.Color("#e6c547")
	ThemeNeutral = lipgloss.Color("#EEF2F3")

	redStyle    = lipgloss.NewStyle().Foreground(ThemeRed)
	blueStyle   = lipgloss.NewStyle().Foreground(ThemeBlue)
	greenStyle  = lipgloss.NewStyle().Foreground(ThemeGreen)
	yellowStyle = lipgloss.NewStyle().Foreground(ThemeYellow)
	boldBlue    = lipgloss.NewStyle().Foreground(ThemeBlue).Bold(true)
	faintStyle  = lipgloss.NewStyle().Faint(true)

	bannerStyle = lipgloss.NewStyle().Foreground(ThemeRed).Padding(1, 2)
	boxStyle    = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ThemeGreen).
			Padding(1, 5)
)

const banner = `
███████╗███╗   ███╗ ██████╗ ███████╗
██╔════╝████╗ ████║██╔═══██╗██╔════╝
█████╗  ██╔████╔██║██║   ██║███████╗
██╔══╝  ██║╚██╔╝██║██║   ██║╚════██║
███████╗██║ ╚═╝ ██║╚██████╔╝███████║
╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚══════╝`

func Banner(version string) {
	fmt.Println(bannerStyle.Render(banner))
	fmt.Println(boldBlue.Render(fmt.Sprintf("  EmbodiedOS Management CLI v%s", version)))
	fmt.Println()
}

func StatusCard(version string) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#d54e53")).
		Padding(0, 1)

	descStyle := lipgloss.NewStyle().
		Foreground(ThemeNeutral).
		PaddingLeft(2)

	linkStyle := lipgloss.NewStyle().
		Foreground(ThemeBlue).
		Underline(true).
		PaddingLeft(2)

	orgStyle := lipgloss.NewStyle().
		Foreground(ThemeYellow).
		Bold(true).
		PaddingLeft(2)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444")).
		Padding(1, 3).
		Width(56)

	content := titleStyle.Render(" EMOS ") + "  " +
		faintStyle.Render("v"+version) + "\n\n" +
		descStyle.Render("The Embodied Operating System") + "\n" +
		descStyle.Render("The automation orchestration layer for your robot.") + "\n\n" +
		orgStyle.Render("Automatika Robotics") + "\n" +
		linkStyle.Render("https://automatikarobotics.com") + "\n" +
		linkStyle.Render("https://github.com/automatika-robotics/emos")

	fmt.Println(card.Render(content))
}

func Header(msg string) {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(ThemeBlue).
		Border(lipgloss.ThickBorder()).
		BorderForeground(ThemeBlue).
		Padding(1, 2)
	fmt.Println(style.Render(msg))
}

func Info(msg string)    { fmt.Printf("  %s %s\n", faintStyle.Render("├─"), msg) }
func Success(msg string) { fmt.Printf("  %s %s\n", greenStyle.Render("✔"), msg) }
func Warn(msg string)    { fmt.Printf("  %s %s\n", yellowStyle.Render("!"), msg) }
func Error(msg string)   { fmt.Printf("  %s %s\n", redStyle.Render("✖"), msg) }
func Faint(msg string)   { fmt.Printf("  %s\n", faintStyle.Render(msg)) }

func SuccessBox(msg string) {
	fmt.Println(boxStyle.Render(msg))
}

func Confirm(prompt string) bool {
	var result bool
	huh.NewConfirm().
		Title(prompt).
		Affirmative("Yes").
		Negative("No").
		Value(&result).
		WithTheme(huhTheme()).
		Run()
	return result
}

func Input(prompt, defaultVal string) string {
	var result string
	huh.NewInput().
		Title(prompt).
		Value(&result).
		Placeholder(defaultVal).
		WithTheme(huhTheme()).
		Run()
	if result == "" {
		return defaultVal
	}
	return result
}

func Spinner(title string, fn func() error) error {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan error, 1)

	go func() {
		done <- fn()
	}()

	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case err := <-done:
			fmt.Print("\r\033[K") // clear line
			if err != nil {
				Error(title)
			} else {
				Success(title)
			}
			return err
		case <-ticker.C:
			fmt.Printf("\r  %s %s", blueStyle.Render(frames[i%len(frames)]), title)
			i++
		}
	}
}

// Select displays an interactive arrow-key menu and returns the 0-based index of the chosen option.
func Select(prompt string, options []string) int {
	var result int
	opts := make([]huh.Option[int], len(options))
	for i, opt := range options {
		opts[i] = huh.NewOption(opt, i)
	}
	huh.NewSelect[int]().
		Title(prompt).
		Options(opts...).
		Value(&result).
		WithTheme(huhTheme()).
		Run()
	return result
}

func huhTheme() *huh.Theme {
	t := huh.ThemeCharm()
	t.Focused.Title = lipgloss.NewStyle().Foreground(ThemeBlue).Bold(true)
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(ThemeGreen)
	t.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(ThemeNeutral)
	return t
}

func PrintTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		Faint("No results.")
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Header
	for i, h := range headers {
		styled := boldBlue.Render(h)
		padding := widths[i] + 3 - len(h)
		fmt.Printf("  %s%s", styled, strings.Repeat(" ", padding))
	}
	fmt.Println()

	// Separator
	for i := range headers {
		fmt.Printf("  %-*s", widths[i]+3, strings.Repeat("─", widths[i]))
	}
	fmt.Println()

	// Rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Printf("  %-*s", widths[i]+3, cell)
			}
		}
		fmt.Println()
	}
}

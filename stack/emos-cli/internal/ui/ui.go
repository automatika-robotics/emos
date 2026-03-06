package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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
	fmt.Printf("  %s [y/N] ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	response := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return response == "y" || response == "yes"
}

func Input(prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s (default: %s): ", prompt, defaultVal)
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return defaultVal
	}
	return val
}

func Spinner(title string, fn func() error) error {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan error, 1)
	var once sync.Once

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
			once.Do(func() {
				if err != nil {
					Error(title)
				} else {
					Success(title)
				}
			})
			return err
		case <-ticker.C:
			fmt.Printf("\r  %s %s", blueStyle.Render(frames[i%len(frames)]), title)
			i++
		}
	}
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

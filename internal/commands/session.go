package commands

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"nexus/internal/backend"
	"nexus/internal/session"
	"nexus/internal/ui"
)

// HandleSessionCommand handles session-related commands.
func (h *Handler) HandleSessionCommand(args []string) {
	if len(args) == 0 {
		h.listSessions()
		return
	}

	subcmd := args[0]
	switch subcmd {
	case "start":
		if len(args) < 2 {
			fmt.Println("Usage: promptops session start <name>")
			return
		}
		h.startSession(args[1])
	case "list":
		h.listSessions()
	case "resume":
		if len(args) < 2 {
			fmt.Println("Usage: promptops session resume <name>")
			return
		}
		h.resumeSession(args[1])
	case "info":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		h.showSessionInfo(name)
	case "close":
		if len(args) < 2 {
			fmt.Println("Usage: promptops session close <name>")
			return
		}
		h.closeSession(args[1])
	case "cleanup":
		h.cleanupSessions()
	default:
		fmt.Printf("Unknown session command: %s\n", subcmd)
	}
}

func (h *Handler) startSession(name string) {
	// Check if session with this name already exists
	sessions := h.sessionMgr.LoadAll()
	for _, s := range sessions {
		if s.Name == name && s.Status != "closed" {
			fmt.Printf("Error: Session '%s' already exists (status: %s)\n", name, s.Status)
			os.Exit(1)
		}
	}

	sess, err := h.sessionMgr.Create(name, h.stateReader.Get())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	be, ok := h.registry.Get(sess.Backend)
	if !ok {
		be = backend.Backend{DisplayName: sess.Backend}
	}
	fmt.Printf("[OK] Started session '%s' with %s backend\n", sess.Name, be.DisplayName)
}

func (h *Handler) listSessions() {
	sessions := h.sessionMgr.LoadAll()
	current := h.sessionMgr.GetCurrent()

	if len(sessions) == 0 {
		fmt.Println("No sessions found. Use 'promptops session start <name>' to create one.")
		return
	}

	fmt.Println()
	fmt.Println(ui.StyleSection.Render("SESSIONS"))

	// Sort by last active (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActive.After(sessions[j].LastActive)
	})

	rows := [][]string{}
	for _, s := range sessions {
		marker := " "
		if current != nil && s.ID == current.ID {
			marker = ui.StyleAccent.Render(">")
		}

		statusStr := s.Status
		switch s.Status {
		case "active":
			statusStr = ui.StyleSuccess.Render(s.Status)
		case "paused":
			statusStr = ui.StyleWarning.Render(s.Status)
		case "closed":
			statusStr = ui.StyleMuted.Render(s.Status)
		}

		started := s.StartTime.Format("01-02 15:04")

		// Safe backend name lookup
		backendName := s.Backend
		if be, ok := h.registry.Get(s.Backend); ok {
			backendName = be.DisplayName
		}

		rows = append(rows, []string{
			marker,
			ui.Truncate(s.Name, 14),
			backendName,
			started,
			fmt.Sprintf("%d", s.PromptCount),
			ui.FormatCurrency(s.TotalCost),
			statusStr,
		})
	}

	t := table.New().
		Headers("", "Name", "Backend", "Started", "Prompts", "Cost", "Status").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary)
			}
			if col == 0 {
				return lipgloss.NewStyle().Width(2)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(90)

	fmt.Println(t.Render())
	fmt.Println()
}

func (h *Handler) resumeSession(name string) {
	sessions := h.sessionMgr.LoadAll()

	for i, s := range sessions {
		if s.Name == name {
			if s.Status == "closed" {
				fmt.Printf("Error: Session '%s' is closed\n", name)
				os.Exit(1)
			}

			sessions[i].Status = "active"
			sessions[i].LastActive = time.Now()
			h.sessionMgr.SaveAll(sessions)
			h.sessionMgr.SetCurrent(s.ID)

			// Also switch to the session's backend
			h.stateWriter.Set(s.Backend)

			// Safe backend name lookup
			backendName := s.Backend
			if be, ok := h.registry.Get(s.Backend); ok {
				backendName = be.DisplayName
			}
			fmt.Printf("[OK] Resumed session '%s' (%s backend)\n", s.Name, backendName)
			return
		}
	}

	fmt.Printf("Error: Session '%s' not found\n", name)
	os.Exit(1)
}

func (h *Handler) showSessionInfo(name string) {
	var sess *session.Session
	if name == "" {
		sess = h.sessionMgr.GetCurrent()
		if sess == nil {
			fmt.Println("No active session. Use 'promptops session info <name>' to show a specific session.")
			os.Exit(1)
		}
	} else {
		sess = h.sessionMgr.FindByName(name)
	}

	if sess == nil {
		fmt.Printf("Error: Session '%s' not found\n", name)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(ui.StyleSection.Render(fmt.Sprintf("SESSION: %s", sess.Name)))
	fmt.Println()

	infoStyle := lipgloss.NewStyle().Width(20).Foreground(ui.ColorSubtle)
	valueStyle := lipgloss.NewStyle()

	fmt.Printf("%s %s\n", infoStyle.Render("ID:"), valueStyle.Render(ui.Truncate(sess.ID, 50)))
	backendName := "Unknown"
	if be, ok := h.registry.Get(sess.Backend); ok {
		backendName = be.DisplayName
	}
	fmt.Printf("%s %s\n", infoStyle.Render("Backend:"), valueStyle.Render(backendName))

	statusStr := sess.Status
	switch sess.Status {
	case "active":
		statusStr = ui.StyleSuccess.Render(sess.Status)
	case "paused":
		statusStr = ui.StyleWarning.Render(sess.Status)
	case "closed":
		statusStr = ui.StyleMuted.Render(sess.Status)
	}
	fmt.Printf("%s %s\n", infoStyle.Render("Status:"), statusStr)

	fmt.Printf("%s %s\n", infoStyle.Render("Started:"), valueStyle.Render(sess.StartTime.Format("2006-01-02 15:04:05")))
	fmt.Printf("%s %s\n", infoStyle.Render("Last Active:"), valueStyle.Render(sess.LastActive.Format("2006-01-02 15:04:05")))
	fmt.Printf("%s %s\n", infoStyle.Render("Working Dir:"), valueStyle.Render(ui.Truncate(sess.WorkingDir, 50)))
	fmt.Printf("%s %s\n", infoStyle.Render("Prompts:"), valueStyle.Render(fmt.Sprintf("%d", sess.PromptCount)))
	fmt.Printf("%s %s\n", infoStyle.Render("Total Cost:"), valueStyle.Render(ui.FormatCurrency(sess.TotalCost)))

	fmt.Println()
}

func (h *Handler) closeSession(name string) {
	if err := h.sessionMgr.Close(name); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] Closed session '%s'\n", name)
}

func (h *Handler) cleanupSessions() {
	removed, err := h.sessionMgr.Cleanup()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if removed > 0 {
		fmt.Printf("[OK] Removed %d old closed sessions\n", removed)
	} else {
		fmt.Println("No old sessions to cleanup")
	}
}

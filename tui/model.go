package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pteich/elastic-query-export/export"
	"github.com/pteich/elastic-query-export/flags"
)

type step int

const (
	stepConnection step = iota
	stepIndex
	stepFields
	stepQuery
	stepExport
	stepProgress
)

type Model struct {
	// State
	step   step
	width  int
	height int
	err    error

	// Data
	conf   *flags.Flags
	client *export.Client

	// Inputs
	inputs []textinput.Model
	focus  int
}

func InitialModel(conf *flags.Flags) Model {
	m := Model{
		step: stepConnection,
		conf: conf,
	}
	m.inputs = make([]textinput.Model, 3)

	var t textinput.Model

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 120
	t.Placeholder = "http://localhost:9200"
	t.SetValue(conf.ElasticURL)
	t.Focus()
	t.Prompt = "URL: "
	t.TextStyle = focusedStyle
	m.inputs[0] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 64
	t.Placeholder = "username"
	t.Prompt = "User: "
	m.inputs[1] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 64
	t.Placeholder = "password"
	t.Prompt = "Pass: "
	t.EchoMode = textinput.EchoPassword
	m.inputs[2] = t

	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			if s == "enter" && m.focus == len(m.inputs)-1 {
				// Initialize connection
				m.conf.ElasticURL = m.inputs[0].Value()
				m.conf.ElasticUser = m.inputs[1].Value()
				m.conf.ElasticPass = m.inputs[2].Value()

				return m, func() tea.Msg {
					client, err := export.NewClient(m.conf)
					if err != nil {
						return errMsg{err}
					}
					// Test connection (maybe get indices)
					indices, err := client.GetIndices(context.Background(), "*")
					if err != nil {
						return errMsg{err}
					}
					return connectedMsg{client: client, indices: indices}
				}
			}

			if s == "up" || s == "shift+tab" {
				m.focus--
			} else {
				m.focus++
			}

			if m.focus > len(m.inputs)-1 {
				m.focus = 0
			} else if m.focus < 0 {
				m.focus = len(m.inputs) - 1
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focus {
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].TextStyle = focusedStyle
					continue
				}
				m.inputs[i].Blur()
				m.inputs[i].TextStyle = noStyle
			}
			return m, tea.Batch(cmds...)
		}

	case connectedMsg:
		m.client = msg.client
		m.step = stepIndex
		// TODO: handle indices
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	// Handle inputs
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress q to quit.", m.err)
	}

	switch m.step {
	case stepConnection:
		var view, inputs string

		for i := range m.inputs {
			inputs += m.inputs[i].View() + "\n"
		}

		view = fmt.Sprintf(
			"Connect to ElasticSearch\n\n%s\n\n[Enter] Connect",
			inputs,
		)
		return view
	case stepIndex:
		return "Connected! Select Index (TODO)"
	default:
		return "Unknown step"
	}
}

// Messages
type connectedMsg struct {
	client  *export.Client
	indices []string
}

type errMsg struct{ err error }

// Styles (temporary until styles.go)
var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	noStyle      = lipgloss.NewStyle()
	helpStyle    = blurredStyle.Copy()
)

package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pteich/elastic-query-export/export"
	"github.com/pteich/elastic-query-export/flags"
)

type step int

const (
	stepConnection step = iota
	stepQueryType
	stepQuery
	stepIndex
	stepIndexManual
	stepFields
	stepFieldsManual
	stepExport
	stepExportManual
	stepProgress
)

type item string

func (i item) FilterValue() string { return string(i) }
func (i item) Title() string       { return string(i) }
func (i item) Description() string { return "" }

type formatItem struct {
	format string
	desc   string
}

func (f formatItem) Title() string       { return f.format }
func (f formatItem) Description() string { return f.desc }
func (f formatItem) FilterValue() string { return f.format }

type queryTypeItem struct {
	queryType string
	desc      string
	field     string
}

func (q queryTypeItem) Title() string       { return q.queryType }
func (q queryTypeItem) Description() string { return q.desc }
func (q queryTypeItem) FilterValue() string { return q.queryType }

type Model struct {
	step   step
	width  int
	height int
	err    error
	done   bool

	conf   *flags.Flags
	client *export.Client

	inputs []textinput.Model
	focus  int
	toggle bool

	list list.Model

	progress       progress.Model
	exported       int
	total          int
	selectedFields map[string]bool
}

func InitialModel(conf *flags.Flags) Model {
	m := Model{
		step:           stepConnection,
		conf:           conf,
		selectedFields: make(map[string]bool),
	}
	m.inputs = make([]textinput.Model, 7)

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
	if conf.ElasticUser != "" {
		t.SetValue(conf.ElasticUser)
	}
	m.inputs[1] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 64
	t.Placeholder = "password"
	t.Prompt = "Pass: "
	t.EchoMode = textinput.EchoPassword
	if conf.ElasticPass != "" {
		t.SetValue(conf.ElasticPass)
	}
	m.inputs[2] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 120
	t.Placeholder = "/path/to/client.crt (optional)"
	t.Prompt = "Client Cert: "
	if conf.ElasticClientCrt != "" {
		t.SetValue(conf.ElasticClientCrt)
	}
	m.inputs[3] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 120
	t.Placeholder = "/path/to/client.key (optional)"
	t.Prompt = "Client Key: "
	if conf.ElasticClientKey != "" {
		t.SetValue(conf.ElasticClientKey)
	}
	m.inputs[4] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 120
	t.Placeholder = "logs-* (supports wildcards)"
	t.Prompt = "Index Pattern: "
	if conf.Index != "" {
		t.SetValue(conf.Index)
	}
	m.inputs[5] = t

	m.toggle = conf.ElasticVerifySSL
	m.conf.ElasticVersion = conf.ElasticVersion

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
		if m.step == stepIndex || m.step == stepExport || m.step == stepQueryType || m.step == stepFields {
			m.list.SetWidth(msg.Width)
			m.list.SetHeight(msg.Height - 6)
		}
		if m.step == stepProgress {
			m.progress = progress.New(progress.WithDefaultGradient())
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.list.FilterState() == list.Filtering {
				m.list.ResetFilter()
				return m, nil
			}
			if m.step == stepProgress {
				return m, tea.Quit
			}
			return m, tea.Quit
		case "left":
			if m.step == stepConnection && m.conf.ElasticVersion > 7 {
				m.conf.ElasticVersion--
			}
		case "right":
			if m.step == stepConnection && m.conf.ElasticVersion < 9 {
				m.conf.ElasticVersion++
			}
		case " ":
			if m.step == stepConnection {
				m.toggle = !m.toggle
				m.conf.ElasticVerifySSL = m.toggle
			} else if m.step == stepFields {
				selected := m.list.SelectedItem()
				if selected != nil {
					field := selected.(item)
					m.selectedFields[string(field)] = !m.selectedFields[string(field)]
				}
			}
		}

		if m.step == stepConnection {
			switch msg.String() {
			case "tab", "shift+tab", "enter", "up", "down":
				s := msg.String()

				if s == "enter" && m.focus == len(m.inputs)-2 {
					m.conf.ElasticURL = m.inputs[0].Value()
					m.conf.ElasticUser = m.inputs[1].Value()
					m.conf.ElasticPass = m.inputs[2].Value()
					m.conf.ElasticClientCrt = m.inputs[3].Value()
					m.conf.ElasticClientKey = m.inputs[4].Value()
					m.conf.ElasticVerifySSL = m.toggle
					m.conf.Index = m.inputs[5].Value()

					m.step = stepQueryType
					m.initQueryTypeList()
					return m, nil
				}

				if s == "up" || s == "shift+tab" {
					m.focus--
				} else {
					m.focus++
				}

				if m.focus > len(m.inputs)-2 {
					m.focus = 0
				} else if m.focus < 0 {
					m.focus = len(m.inputs) - 2
				}

				cmds := make([]tea.Cmd, len(m.inputs))
				for i := 0; i <= len(m.inputs)-2; i++ {
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
		} else if m.step == stepQueryType {
			switch msg.String() {
			case "enter":
				if m.list.FilterState() == list.Filtering {
					break
				}
				q, ok := m.list.SelectedItem().(queryTypeItem)
				if ok {
					if q.field != "query_placeholder" {
						m.conf.Query = q.field
					}
					if q.field == "" {
						m.conf.RAWQuery = ""
					}
					m.step = stepQuery
					m.initQueryInputs()
					return m, nil
				}
			}
		} else if m.step == stepQuery {
			switch msg.String() {
			case "enter":
				if m.focus == len(m.inputs)-1 {
					m.conf.Query = m.inputs[0].Value()
					m.conf.RAWQuery = m.inputs[1].Value()
					m.conf.StartDate = m.inputs[2].Value()
					m.conf.EndDate = m.inputs[3].Value()
					m.conf.Timefield = m.inputs[4].Value()
					m.conf.ScrollSize = 1000
					m.conf.Index = m.inputs[5].Value()

					return m, func() tea.Msg {
						client, err := export.NewClient(m.conf)
						if err != nil {
							return errMsg{err}
						}
						indices, err := client.GetIndices(context.Background(), m.inputs[5].Value())
						if err != nil {
							return errMsg{err}
						}
						return connectedMsg{client: client, indices: indices, indexPattern: m.inputs[5].Value()}
					}
				}

				if msg.String() == "up" || msg.String() == "shift+tab" {
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
		} else if m.step == stepIndex {
			switch msg.String() {
			case "m":
				m.step = stepIndexManual
				m.initIndexManualInput()
				return m, nil
			case "enter":
				if m.list.FilterState() == list.Filtering {
					break
				}
				i, ok := m.list.SelectedItem().(item)
				if ok {
					m.conf.Index = string(i)
					m.step = stepFields
					m.initFieldsList()
					return m, nil
				}
			}
		} else if m.step == stepIndexManual {
			switch msg.String() {
			case "enter":
				m.conf.Index = m.inputs[0].Value()
				m.step = stepFields
				m.initFieldsList()
				return m, nil
			case "esc":
				m.step = stepIndex
				return m, nil
			}
		} else if m.step == stepFields {
			switch msg.String() {
			case "m":
				m.step = stepFieldsManual
				m.initFieldsManualInput()
				return m, nil
			case "enter":
				if len(m.selectedFields) > 0 {
					fields := make([]string, 0, len(m.selectedFields))
					for field, selected := range m.selectedFields {
						if selected {
							fields = append(fields, field)
						}
					}
					m.conf.Fields = fields
					m.conf.Fieldlist = strings.Join(fields, ",")
				}
				m.step = stepExport
				m.initExportList()
				return m, nil
			}
		} else if m.step == stepFieldsManual {
			switch msg.String() {
			case "enter":
				m.conf.Fieldlist = m.inputs[0].Value()
				if m.inputs[0].Value() != "" {
					m.conf.Fields = strings.Split(m.inputs[0].Value(), ",")
				}
				m.step = stepExport
				m.initExportList()
				return m, nil
			case "esc":
				m.step = stepFields
				return m, nil
			}
		} else if m.step == stepExport {
			switch msg.String() {
			case "m":
				m.step = stepExportManual
				m.initExportManualInput()
				return m, nil
			case "enter":
				if m.list.FilterState() == list.Filtering {
					break
				}
				f, ok := m.list.SelectedItem().(formatItem)
				if ok {
					m.conf.OutFormat = f.format
					m.conf.Outfile = "output." + f.format
					m.step = stepProgress
					return m, startExportCmd(m)
				}
			}
		} else if m.step == stepExportManual {
			switch msg.String() {
			case "enter":
				if m.focus == len(m.inputs)-1 {
					m.conf.OutFormat = m.inputs[0].Value()
					m.conf.Outfile = m.inputs[1].Value()
					m.step = stepProgress
					return m, startExportCmd(m)
				}

				if msg.String() == "up" || msg.String() == "shift+tab" {
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
		}

	case exportProgressMsg:
		m.exported = msg.exported
		m.total = msg.total
		if msg.done {
			m.done = true
			return m, tea.Quit
		}

	case errMsg:
		m.err = msg.err
		return m, nil

	case connectedMsg:
		m.client = msg.client
		m.conf.Index = msg.indexPattern

		if len(msg.indices) > 0 {
			m.step = stepIndex
			var items []list.Item
			for _, i := range msg.indices {
				items = append(items, item(i))
			}
			m.list = list.New(items, list.NewDefaultDelegate(), m.width, m.height-6)
			m.list.Title = "Select Index (or 'm' for manual pattern)"
			m.list.SetShowStatusBar(false)
		} else {
			m.step = stepIndexManual
			m.initIndexManualInput()
		}
		return m, nil
	}

	var cmd tea.Cmd
	if m.step == stepConnection {
		cmds := make([]tea.Cmd, len(m.inputs))
		for i := range m.inputs {
			m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
		}
		return m, tea.Batch(cmds...)
	} else if m.step == stepIndex {
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	} else if m.step == stepIndexManual {
		cmds := make([]tea.Cmd, len(m.inputs))
		for i := range m.inputs {
			m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
		}
		return m, tea.Batch(cmds...)
	} else if m.step == stepFields {
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	} else if m.step == stepFieldsManual {
		cmds := make([]tea.Cmd, len(m.inputs))
		for i := range m.inputs {
			m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
		}
		return m, tea.Batch(cmds...)
	} else if m.step == stepExport {
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	} else if m.step == stepExportManual {
		cmds := make([]tea.Cmd, len(m.inputs))
		for i := range m.inputs {
			m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
		}
		return m, tea.Batch(cmds...)
	} else if m.step == stepQuery {
		cmds := make([]tea.Cmd, len(m.inputs))
		for i := range m.inputs {
			m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
		}
		return m, tea.Batch(cmds...)
	} else if m.step == stepQueryType {
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	} else if m.step == stepProgress {
		var progressCmd tea.Cmd
		progressModel, progressCmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, progressCmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress Ctrl+C or Esc to quit.", m.err)
	}

	switch m.step {
	case stepConnection:
		var view, inputs string

		for i := 0; i <= len(m.inputs)-2; i++ {
			inputs += m.inputs[i].View() + "\n"
		}

		sslStatus := "[ ]"
		if m.toggle {
			sslStatus = "[X]"
		}

		versionStatus := fmt.Sprintf("Version: [%d] [←/→ to change, use 7 for OpenSearch]", m.conf.ElasticVersion)

		view = fmt.Sprintf(
			"Connection Settings\n\n%s\n%s\n%s\n\n[Tab/Enter] Next field  [←/→] Change version  [Space] Toggle SSL  [Enter on URL] Continue",
			inputs,
			sslStatus,
			versionStatus,
		)
		return view
	case stepQueryType:
		return m.list.View()
	case stepQuery:
		var inputs string

		for i := 0; i <= len(m.inputs)-1; i++ {
			inputs += m.inputs[i].View() + "\n"
		}

		return fmt.Sprintf(
			"Query Settings\n\n%s\n\n[Tab/Enter] Next field  [Enter] Continue",
			inputs,
		)
	case stepIndex:
		return m.list.View()
	case stepIndexManual:
		return fmt.Sprintf(
			"Manual Index Pattern\n\n%s\n\n[Enter] Continue  [Esc] Back to list",
			m.inputs[0].View(),
		)
	case stepFields:
		var listContent string
		items := m.list.Items()
		for _, it := range items {
			field := it.(item)
			prefix := " "
			if m.selectedFields[string(field)] {
				prefix = "[X]"
			}
			listContent += fmt.Sprintf("%s %s\n", prefix, string(field))
		}
		return fmt.Sprintf(
			"Select Fields (or 'm' for manual)\n\n%s\n\n[Space] Select/Deselect  [Enter] Continue  [m] Manual input",
			listContent,
		)
	case stepFieldsManual:
		return fmt.Sprintf(
			"Manual Field List\n\n%s\n\n[Enter] Continue  [Esc] Back to selection",
			m.inputs[0].View(),
		)
	case stepExport:
		return fmt.Sprintf(
			"Select Export Format (or 'm' for manual)\n\n%s\n\n[Enter] Select  [m] Manual configuration",
			m.list.View(),
		)
	case stepExportManual:
		var inputs string
		for i := 0; i <= len(m.inputs)-1; i++ {
			inputs += m.inputs[i].View() + "\n"
		}
		return fmt.Sprintf(
			"Manual Export Configuration\n\n%s\n\n[Tab/Enter] Next field  [Enter] Start export",
			inputs,
		)
	case stepProgress:
		percent := float64(m.exported) / float64(m.total)
		if m.total == 0 {
			percent = 0
		}
		return fmt.Sprintf(
			"Exporting...\n\n%s\n\n%d / %d documents\n\nPress Esc to cancel",
			m.progress.ViewAs(percent),
			m.exported,
			m.total,
		)
	default:
		return "Unknown step"
	}
}

func (m *Model) initQueryTypeList() {
	items := []list.Item{
		queryTypeItem{ty: "Match All", desc: "Return all documents", field: "*"},
		queryTypeItem{ty: "Lucene Query", desc: "Query string like in Kibana", field: "query_placeholder"},
		queryTypeItem{ty: "Raw JSON Query", desc: "Raw Elasticsearch query DSL", field: ""},
	}

	m.list = list.New(items, list.NewDefaultDelegate(), m.width, m.height-6)
	m.list.Title = "Select Query Type"
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(false)
}

func (m *Model) initQueryInputs() {
	m.inputs = make([]textinput.Model, 6)

	var t textinput.Model

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 256
	t.Placeholder = "*"
	t.Prompt = "Query: "
	t.Focus()
	t.TextStyle = focusedStyle
	m.inputs[0] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 1000
	t.Placeholder = `{"query": {"match_all": {}}}`
	t.Prompt = "RAW Query: "
	m.inputs[1] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 64
	t.Placeholder = "2024-01-01"
	t.Prompt = "Start Date: "
	m.inputs[2] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 64
	t.Placeholder = "2024-12-31"
	t.Prompt = "End Date: "
	m.inputs[3] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 64
	t.Placeholder = "@timestamp"
	t.Prompt = "Timefield: "
	t.SetValue("@timestamp")
	m.inputs[4] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 120
	t.Placeholder = "logs-*"
	t.Prompt = "Index Pattern: "
	t.SetValue(m.conf.Index)
	m.inputs[5] = t

	m.focus = 0
}

func (m *Model) initIndexManualInput() {
	m.inputs = make([]textinput.Model, 1)

	t := textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 120
	t.Placeholder = "logs-*"
	t.Prompt = "Index Pattern: "
	t.SetValue(m.conf.Index)
	t.Focus()
	t.TextStyle = focusedStyle
	m.inputs[0] = t
}

func (m *Model) initFieldsList() {
	items := []list.Item{
		item("@timestamp"),
		item("message"),
		item("level"),
		item("host"),
		item("service"),
		item("tags"),
		item("_source"),
		item("_id"),
		item("_index"),
	}

	m.list = list.New(items, list.NewDefaultDelegate(), m.width, m.height-6)
	m.list.Title = "Select Fields (use Space to select multiple)"
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(true)
	m.selectedFields = make(map[string]bool)
}

func (m *Model) initFieldsManualInput() {
	m.inputs = make([]textinput.Model, 1)

	t := textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 500
	t.Placeholder = "field1,field2,field3"
	t.Prompt = "Fields (comma separated): "
	if m.conf.Fieldlist != "" {
		t.SetValue(m.conf.Fieldlist)
	}
	t.Focus()
	t.TextStyle = focusedStyle
	m.inputs[0] = t
}

func (m *Model) initExportList() {
	items := []list.Item{
		formatItem{format: "csv", desc: "CSV format with headers"},
		formatItem{format: "json", desc: "JSON lines format"},
		formatItem{format: "raw", desc: "Raw Elasticsearch format"},
	}

	m.list = list.New(items, list.NewDefaultDelegate(), m.width, m.height-6)
	m.list.Title = "Select Output Format"
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(false)
}

func (m *Model) initExportManualInput() {
	m.inputs = make([]textinput.Model, 2)

	t := textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 10
	t.Placeholder = "csv"
	t.Prompt = "Format: "
	t.SetValue(m.conf.OutFormat)
	t.Focus()
	t.TextStyle = focusedStyle
	m.inputs[0] = t

	t = textinput.New()
	t.Cursor.Style = cursorStyle
	t.CharLimit = 120
	t.Placeholder = "output.csv"
	t.Prompt = "Output File: "
	t.SetValue(m.conf.Outfile)
	m.inputs[1] = t

	m.focus = 0
}

type connectedMsg struct {
	client       *export.Client
	indices      []string
	indexPattern string
}

type errMsg struct{ err error }

type exportProgressMsg struct {
	exported int
	total    int
	done     bool
}

func startExportCmd(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())

		total, err := m.client.Count(ctx, m.conf.Index, export.BuildQuery(m.conf.ElasticVersion, m.conf))
		if err != nil {
			return errMsg{err}
		}

		go func() {
			defer cancel()
			export.Run(ctx, m.conf)
		}()

		time.Sleep(100 * time.Millisecond)

		return exportProgressMsg{exported: 0, total: int(total), done: true}
	}
}

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	noStyle      = lipgloss.NewStyle()
)

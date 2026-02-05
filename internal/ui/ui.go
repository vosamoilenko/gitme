package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vosamoilenko/gitme/internal/identity"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	currentStyle      = lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("240"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginLeft(2)
	deleteStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// Action represents what the user wants to do
type Action int

const (
	ActionNone Action = iota
	ActionSelect
	ActionDelete
	ActionRescan
)

// item wraps an identity for the list
type item struct {
	identity  identity.Identity
	isCurrent bool
}

func (i item) FilterValue() string { return i.identity.Email }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s <%s>", i.identity.Name, i.identity.Email)
	if i.isCurrent {
		str += " (current)"
	}

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + s[0])
		}
	} else if i.isCurrent {
		fn = currentStyle.Render
	}

	fmt.Fprint(w, fn(str))
}

// Model is the main UI model
type Model struct {
	list           list.Model
	choice         *identity.Identity
	action         Action
	quitting       bool
	folder         string
	confirmDelete  bool
	deleteTarget   *identity.Identity
}

// New creates a new UI model
func New(identities []identity.Identity, currentIdentity *identity.Identity, folder string) Model {
	items := make([]list.Item, len(identities))
	for i, id := range identities {
		isCurrent := currentIdentity != nil && id.Email == currentIdentity.Email
		items[i] = item{identity: id, isCurrent: isCurrent}
	}

	l := list.New(items, itemDelegate{}, 50, 14)
	l.Title = "gitme"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.SetShowHelp(false)

	return Model{
		list:   l,
		folder: folder,
		action: ActionNone,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		// Handle delete confirmation
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				m.action = ActionDelete
				return m, tea.Quit
			case "n", "N", "esc":
				m.confirmDelete = false
				m.deleteTarget = nil
				return m, nil
			}
			return m, nil
		}

		// Don't capture keys when filtering
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if i, ok := m.list.SelectedItem().(item); ok {
				m.choice = &i.identity
				m.action = ActionSelect
			}
			return m, tea.Quit

		case "d", "x":
			if i, ok := m.list.SelectedItem().(item); ok {
				m.deleteTarget = &i.identity
				m.confirmDelete = true
			}
			return m, nil

		case "r":
			m.action = ActionRescan
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.confirmDelete && m.deleteTarget != nil {
		return fmt.Sprintf("\n  %s\n\n  %s\n\n  %s\n",
			deleteStyle.Render("Delete identity?"),
			fmt.Sprintf("  %s <%s>", m.deleteTarget.Name, m.deleteTarget.Email),
			helpStyle.Render("y: yes • n: no"),
		)
	}

	return "\n" + m.list.View() + "\n" + helpStyle.Render("  ↑/↓: navigate • enter: select • d: delete • r: rescan • /: filter • q: quit") + "\n"
}

// Choice returns the selected identity
func (m Model) Choice() *identity.Identity {
	return m.choice
}

// Action returns what action the user wants to take
func (m Model) Action() Action {
	return m.action
}

// DeleteTarget returns the identity to delete
func (m Model) DeleteTarget() *identity.Identity {
	return m.deleteTarget
}

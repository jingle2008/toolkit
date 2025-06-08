// Package toolkit: update_list.go
// Contains updateListView and related list view logic split from model_update.go.

package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/domain"
)

func updateListView(msg tea.Msg, m *Model) (tea.Model, tea.Cmd) { //nolint:gocognit,cyclop
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.loading {
			break
		}

		if msg.String() == "esc" {
			m.backToLastState()
		}

		if m.mode == Normal {
			switch {

			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit

			case key.Matches(msg, m.keys.NextCategory):
				next := int(m.category) + 1
				if next > int(domain.DedicatedAICluster) {
					next = int(domain.Tenant)
				}
				category := domain.Category(next)
				cmds = append(cmds, m.updateCategory(category))

			case key.Matches(msg, m.keys.PrevCategory):
				prev := int(m.category) - 1
				if prev < int(domain.Tenant) {
					prev = int(domain.DedicatedAICluster)
				}
				category := domain.Category(prev)
				cmds = append(cmds, m.updateCategory(category))

			case key.Matches(msg, m.keys.FilterItems):
				m.enterEditMode(Filter)

			case key.Matches(msg, m.keys.JumpTo):
				m.enterEditMode(Alias)

			case key.Matches(msg, m.keys.ViewDetails):
				m.enterDetailView()

			case key.Matches(msg, m.keys.ApplyContext):
				cmd = m.enterContext()
				cmds = append(cmds, cmd)

			default:
				m.handleAdditionalKeys(msg)
			}
		} else {
			updatedTextInput, cmd := m.textInput.Update(msg)
			m.textInput = &updatedTextInput
			cmds = append(cmds, cmd)

			switch msg.String() {

			case "enter":
				if m.target == Alias {
					cmd = m.changeCategory()
					if cmd == nil {
						break
					}
					cmds = append(cmds, cmd)
				}
				m.exitEditMode(m.target == Alias)

			case "esc":
				m.exitEditMode(true)

			default:
				if m.target == Filter {
					cmds = append(cmds, DebounceFilter(m))
				}
			}
		}

	case DataMsg:
		m.processData(msg)

	case FilterMsg:
		if msg.Text == m.newFilter {
			FilterTable(m, msg.Text)
		}

	case ErrMsg:
		m.err = msg.Err
	}

	updatedTable, cmd := m.table.Update(msg)
	m.table = &updatedTable
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

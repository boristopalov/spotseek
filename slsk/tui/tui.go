package tui

import (
	"fmt"
	"spotseek/slsk/client"
	"spotseek/slsk/peer"
	"spotseek/slsk/shared"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type SearchResultRow struct {
	Username   string
	Filename   string
	Size       string
	BitRate    string
	SlotStatus string
}

type DownloadRow struct {
	Username string
	Filename string
	Progress string
	Status   string
}

type model struct {
	client        *client.SlskClient
	searchInput   textinput.Model
	table         table.Model
	downloadTable table.Model
	rows          []SearchResultRow
	downloads     []DownloadRow
	activeView    string // "search" or "downloads"
	err           error
}

func NewModel(client *client.SlskClient) model {
	ti := textinput.New()
	ti.Placeholder = "Enter search term..."
	ti.Focus()

	columns := []table.Column{
		{Title: "Username", Width: 20},
		{Title: "Filename", Width: 40},
		{Title: "Size", Width: 10},
		{Title: "BitRate", Width: 10},
		{Title: "Status", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	downloadColumns := []table.Column{
		{Title: "Username", Width: 20},
		{Title: "Filename", Width: 40},
		{Title: "Size", Width: 10},
		{Title: "Progress", Width: 10},
		{Title: "Status", Width: 10},
	}

	dt := table.New(
		table.WithColumns(downloadColumns),
		table.WithFocused(false),
		table.WithHeight(5),
	)

	return model{
		client:        client,
		searchInput:   ti,
		table:         t,
		downloadTable: dt,
		activeView:    "search",
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle global keys first
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyTab:
			// Toggle between search input, search results, and downloads view
			if m.searchInput.Focused() {
				// Move from search input to search results table
				m.searchInput.Blur()
				m.table.Focus()
				return m, nil
			} else if m.activeView == "search" && m.table.Focused() {
				// Move from search results to downloads
				m.activeView = "downloads"
				m.table.Blur()
				m.downloadTable.Focus()
				return m, nil
			} else {
				// Move from downloads back to search input
				m.activeView = "search"
				m.downloadTable.Blur()
				m.searchInput.Focus()
				return m, nil
			}
		}

		// Handle context-specific keys
		if m.searchInput.Focused() {
			// When search input is focused
			switch msg.Type {
			case tea.KeyEnter:
				if m.searchInput.Value() != "" {
					m.client.FileSearch(m.searchInput.Value())
					m.searchInput.Reset()
				}
			default:
				// Pass other keys to the search input
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			}
		} else if m.activeView == "search" && m.table.Focused() {
			// When search results table is focused
			switch msg.Type {
			case tea.KeyEnter:
				if len(m.rows) > 0 {
					selectedRow := m.table.Cursor()
					if selectedRow < len(m.rows) {
						row := m.rows[selectedRow]
						m.AddDownload(row.Username, row.Filename)
					}
				}
			default:
				// Pass other keys to the table
				m.table, cmd = m.table.Update(msg)
				return m, cmd
			}
		} else if m.activeView == "downloads" && m.downloadTable.Focused() {
			// When downloads table is focused
			m.downloadTable, cmd = m.downloadTable.Update(msg)
			return m, cmd
		}

	case peer.PeerEvent:
		if msg.Type == peer.FileSearchResponse {
			data := msg.Data.(peer.FileSearchData)
			m.UpdateResults(data.Results)
		}
	}

	return m, cmd
}

func (m *model) AddDownload(username string, filename string) {
	// Check if this file is already being downloaded
	for _, d := range m.downloads {
		if d.Username == username && d.Filename == filename {
			// File is already in download queue, don't add it again
			return
		}
	}

	// Add to downloads list
	download := DownloadRow{
		Username: username,
		Filename: filename,
		Progress: "0%",
		Status:   "Pending",
	}
	m.downloads = append(m.downloads, download)

	// Update download table
	m.updateDownloadTableRows()
	// return

	// Request the download from the peer
	go func() {
		peer := m.client.PeerManager.GetPeer(username)
		if peer != nil {
			peer.QueueUpload(filename)
		} else {
			// Update status if peer not found
			for i, d := range m.downloads {
				if d.Username == username && d.Filename == filename {
					m.downloads[i].Status = "Error: Peer not found"
					m.updateDownloadTableRows()
					break
				}
			}
		}
	}()
}

func (m *model) updateDownloadTableRows() {
	var tableRows []table.Row
	for _, d := range m.downloads {
		tableRows = append(tableRows, table.Row{
			d.Username,
			d.Filename,
			d.Progress,
			d.Status,
		})
	}
	m.downloadTable.SetRows(tableRows)
}

func (m *model) updateTableRows() {
	var tableRows []table.Row
	for _, r := range m.rows {
		tableRows = append(tableRows, table.Row{
			r.Username,
			r.Filename,
			r.Size,
			r.BitRate,
			r.SlotStatus,
		})
	}
	m.table.SetRows(tableRows)
}

func (m *model) UpdateResults(result shared.SearchResult) {
	for _, file := range result.PublicFiles {
		row := SearchResultRow{
			Username:   result.Username,
			Filename:   file.Name,
			Size:       formatSize(file.Size),
			BitRate:    fmt.Sprintf("%dkbps", file.BitRate),
			SlotStatus: formatSlotStatus(result.SlotFree),
		}
		m.rows = append(m.rows, row)
	}

	// Update table rows
	m.updateTableRows()
}

func (m model) View() string {
	// Style for the active component
	activeStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)

	// Style for inactive components
	inactiveStyle := baseStyle

	// Apply appropriate styles based on focus
	var searchInputView, searchTableView, downloadsTableView string

	if m.searchInput.Focused() {
		searchInputView = activeStyle.Render(m.searchInput.View())
	} else {
		searchInputView = m.searchInput.View()
	}

	if m.activeView == "search" && m.table.Focused() {
		searchTableView = activeStyle.Render(m.table.View())
	} else {
		searchTableView = inactiveStyle.Render(m.table.View())
	}

	if m.activeView == "downloads" && m.downloadTable.Focused() {
		downloadsTableView = activeStyle.Render(m.downloadTable.View())
	} else {
		downloadsTableView = inactiveStyle.Render(m.downloadTable.View())
	}

	searchView := lipgloss.JoinVertical(lipgloss.Left,
		"Search Soulseek (Tab to cycle between components)",
		searchInputView,
		searchTableView,
		"Enter: Search/Download | Tab: Switch focus | ↑/↓: Navigate",
	)

	downloadsView := lipgloss.JoinVertical(lipgloss.Left,
		"Downloads",
		downloadsTableView,
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		searchView,
		downloadsView,
	)
}

func formatSize(size uint64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := unit, 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func formatSlotStatus(slotFree uint8) string {
	if slotFree > 0 {
		return "Available"
	}
	return "Busy"
}

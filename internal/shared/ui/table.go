package ui

import (
	"github.com/charmbracelet/lipgloss"
)

type TableColumn struct {
	Title string
	Width int
}

type Table struct {
	Columns   []TableColumn
	Rows      [][]string
	Selected  int
	colWidths []int
}

func NewTable(columns []TableColumn) *Table {
	return &Table{
		Columns: columns,
		Rows:    make([][]string, 0),
	}
}

func (t *Table) AddRow(row []string) {
	t.Rows = append(t.Rows, row)
}

func (t *Table) SetRows(rows [][]string) {
	t.Rows = rows
}

func (t *Table) SetSelected(idx int) {
	if idx >= 0 && idx < len(t.Rows) {
		t.Selected = idx
	}
}

func (t *Table) View() string {
	if len(t.Columns) == 0 {
		return ""
	}

	colWidths := make([]int, len(t.Columns))
	for i, col := range t.Columns {
		colWidths[i] = col.Width
	}

	for _, row := range t.Rows {
		for i, cell := range row {
			if i >= len(colWidths) {
				break
			}
			cellWidth := lipgloss.Width(cell)
			if cellWidth > colWidths[i] {
				colWidths[i] = cellWidth
			}
		}
	}

	headerCells := make([]string, len(t.Columns))
	for i, col := range t.Columns {
		cell := lipgloss.NewStyle().
			Width(colWidths[i]).
			Bold(true).
			Foreground(AccentColor).
			Render(col.Title)
		headerCells[i] = cell
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, headerCells...)

	var rows []string
	for rowIdx, row := range t.Rows {
		rowCells := make([]string, len(t.Columns))
		for i := 0; i < len(t.Columns); i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			style := lipgloss.NewStyle().
				Width(colWidths[i])
			if rowIdx == t.Selected {
				style = style.Foreground(HighlightColor).Bold(true)
			}
			rowCells[i] = style.Render(cell)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCells...))
	}

	result := header + "\n"
	for _, row := range rows {
		result += row + "\n"
	}

	return result
}

func (t *Table) SelectedRow() []string {
	if t.Selected >= 0 && t.Selected < len(t.Rows) {
		return t.Rows[t.Selected]
	}
	return nil
}

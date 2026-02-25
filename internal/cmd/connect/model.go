package connect

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type pane int
type overlay int

const (
	paneBuckets pane = iota
	paneObjects
)

const (
	overlayNone overlay = iota
	overlayPalette
	overlayProperties
)

type model struct {
	client       *s3.Client
	program      *tea.Program
	activePane   pane
	overlay      overlay
	buckets      []string
	objects      []S3Entry
	cursorBucket int
	cursorObject int
	offsetBucket int
	offsetObject int
	bucket       string
	prefix       string
	history      []string
	width        int
	height       int
	err          error
	loading      bool
	spinner      spinner.Model

	taskHistory []string

	propEntry *S3Entry

	downloading bool
	dlProgress  progress.Model
	dlName      string
	dlError     error
	dlStatus    string

	uploading  bool
	upProgress progress.Model
	upName     string
	upError    error
	upStatus   string

	help help.Model
	keys keyMap
}

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Back       key.Binding
	Tab        key.Binding
	Quit       key.Binding
	CmdPalette key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Home       key.Binding
	End        key.Binding
	Upload     key.Binding
	Delete     key.Binding
	Refresh    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Tab, k.Back, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Tab, k.Back},
		{k.Home, k.End, k.PageUp, k.PageDown},
		{k.Refresh, k.Upload, k.Delete, k.Quit},
	}
}

var keys = keyMap{
	Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k", "up")),
	Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j", "down")),
	Enter:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Back:       key.NewBinding(key.WithKeys("backspace"), key.WithHelp("back", "back")),
	Tab:        key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	CmdPalette: key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("^p", "commands")),
	PageUp:     key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	PageDown:   key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
	Home:       key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
	End:        key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
	Upload:     key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "upload")),
	Delete:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
}

func initialModel(client *s3.Client) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return model{
		client:      client,
		activePane:  paneBuckets,
		overlay:     overlayNone,
		help:        help.New(),
		keys:        keys,
		dlProgress:  progress.New(progress.WithDefaultGradient()),
		upProgress:  progress.New(progress.WithDefaultGradient()),
		spinner:     s,
		taskHistory: []string{"TUI started"},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.loadBuckets, m.spinner.Tick)
}

func (m model) loadBuckets() tea.Msg {
	buckets, err := listBuckets(context.Background(), m.client)
	if err != nil {
		return err
	}
	return bucketsMsg(buckets)
}

func (m model) loadObjects() tea.Msg {
	objects, err := listObjects(context.Background(), m.client, m.bucket, m.prefix)
	if err != nil {
		return err
	}
	return objectsMsg(objects)
}

func (m model) loadMetadata(bucket, key string) tea.Cmd {
	return func() tea.Msg {
		meta, err := getObjectMetadata(context.Background(), m.client, bucket, key)
		if err != nil {
			return err
		}
		return propsMsg{meta}
	}
}

type bucketsMsg []string
type objectsMsg []S3Entry
type propsMsg struct{ meta *S3Entry }
type dlProgressMsg float64
type dlDoneMsg struct{ err error }
type clearStatusMsg struct{}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	paneHeight := m.getViewHeight()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.overlay != overlayNone {
			if msg.String() == "esc" || msg.String() == "q" {
				m.overlay = overlayNone
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Home):
			if m.activePane == paneBuckets {
				m.cursorBucket = 0
				m.offsetBucket = 0
			} else {
				m.cursorObject = 0
				m.offsetObject = 0
			}
			return m, nil

		case key.Matches(msg, m.keys.End):
			if m.activePane == paneBuckets {
				m.cursorBucket = len(m.buckets) - 1
				m.offsetBucket = m.cursorBucket - paneHeight + 1
				if m.offsetBucket < 0 {
					m.offsetBucket = 0
				}
			} else {
				m.cursorObject = len(m.objects) - 1
				m.offsetObject = m.cursorObject - paneHeight + 2
				if m.offsetObject < 0 {
					m.offsetObject = 0
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.PageUp):
			if m.activePane == paneBuckets {
				m.cursorBucket -= paneHeight
				if m.cursorBucket < 0 {
					m.cursorBucket = 0
				}
				m.offsetBucket = m.cursorBucket
			} else {
				m.cursorObject -= paneHeight
				if m.cursorObject < 0 {
					m.cursorObject = 0
				}
				m.offsetObject = m.cursorObject
			}
			return m, nil

		case key.Matches(msg, m.keys.PageDown):
			if m.activePane == paneBuckets {
				m.cursorBucket += paneHeight
				if m.cursorBucket >= len(m.buckets) {
					m.cursorBucket = len(m.buckets) - 1
				}
				m.offsetBucket = m.cursorBucket - paneHeight + 1
				if m.offsetBucket < 0 {
					m.offsetBucket = 0
				}
			} else {
				m.cursorObject += paneHeight
				if m.cursorObject >= len(m.objects) {
					m.cursorObject = len(m.objects) - 1
				}
				m.offsetObject = m.cursorObject - paneHeight + 2
				if m.offsetObject < 0 {
					m.offsetObject = 0
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.CmdPalette):
			m.overlay = overlayPalette
			return m, nil

		case key.Matches(msg, m.keys.Tab):
			if m.overlay == overlayNone {
				if m.activePane == paneBuckets {
					m.activePane = paneObjects
				} else {
					m.activePane = paneBuckets
				}
			}

		case key.Matches(msg, m.keys.Up):
			if m.activePane == paneBuckets {
				if m.cursorBucket > 0 {
					m.cursorBucket--
					if m.cursorBucket < m.offsetBucket {
						m.offsetBucket = m.cursorBucket
					}
				}
			} else {
				if m.cursorObject > 0 {
					m.cursorObject--
					if m.cursorObject < m.offsetObject {
						m.offsetObject = m.cursorObject
					}
				}
			}

		case key.Matches(msg, m.keys.Down):
			if m.activePane == paneBuckets {
				if m.cursorBucket < len(m.buckets)-1 {
					m.cursorBucket++
					if m.cursorBucket >= m.offsetBucket+paneHeight {
						m.offsetBucket = m.cursorBucket - paneHeight + 1
					}
				}
			} else {
				if m.cursorObject < len(m.objects)-1 {
					m.cursorObject++
					if m.cursorObject >= m.offsetObject+paneHeight-1 {
						m.offsetObject = m.cursorObject - paneHeight + 2
					}
				}
			}

		case key.Matches(msg, m.keys.Enter):
			if m.activePane == paneBuckets {
				if len(m.buckets) > 0 {
					m.bucket = m.buckets[m.cursorBucket]
					m.prefix = ""
					m.history = nil
					m.activePane = paneObjects
					m.offsetObject = 0
					m.cursorObject = 0
					m.loading = true
					return m, m.loadObjects
				}
			} else {
				if len(m.objects) > 0 {
					obj := m.objects[m.cursorObject]
					if obj.IsDir {
						m.history = append(m.history, m.prefix)
						m.prefix += obj.Name
						m.cursorObject = 0
						m.offsetObject = 0
						m.loading = true
						return m, m.loadObjects
					} else {
						m.addHistory(fmt.Sprintf("Download started: %s", obj.Name))
						return m, m.startDownload(obj)
					}
				}
			}

		case key.Matches(msg, m.keys.Back):
			if m.activePane == paneObjects {
				if len(m.history) > 0 {
					m.prefix = m.history[len(m.history)-1]
					m.history = m.history[:len(m.history)-1]
					m.cursorObject = 0
					m.offsetObject = 0
					m.loading = true
					return m, m.loadObjects
				} else {
					m.activePane = paneBuckets
				}
			}

		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			if m.activePane == paneBuckets || m.bucket == "" {
				return m, m.loadBuckets
			}
			return m, m.loadObjects

		case key.Matches(msg, m.keys.Upload):
			if m.bucket != "" {
				m.addHistory("Upload: Use CLI 's3-client upload' command")
			}

		case key.Matches(msg, m.keys.Delete):
			if m.activePane == paneObjects && len(m.objects) > 0 {
				obj := m.objects[m.cursorObject]
				m.addHistory(fmt.Sprintf("Delete: Use CLI to delete s3://%s/%s%s", m.bucket, m.prefix, obj.Name))
			}
		}

	case bucketsMsg:
		m.buckets = msg
		m.loading = false

	case objectsMsg:
		m.objects = msg
		m.loading = false

	case propsMsg:
		m.propEntry = msg.meta
		m.overlay = overlayProperties
		m.loading = false
		return m, nil

	case error:
		m.err = msg
		m.loading = false
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		return m, nil

	case dlProgressMsg:
		cmd := m.dlProgress.SetPercent(float64(msg))
		return m, cmd

	case dlDoneMsg:
		m.downloading = false
		m.dlError = msg.err
		if msg.err != nil {
			m.dlStatus = fmt.Sprintf("Error downloading %s: %v", m.dlName, msg.err)
		} else {
			m.dlStatus = fmt.Sprintf("Successfully downloaded %s", m.dlName)
		}
		m.addHistory(m.dlStatus)
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		})

	case clearStatusMsg:
		m.dlStatus = ""
		return m, nil

	case progress.FrameMsg:
		progressModel, cmd := m.dlProgress.Update(msg)
		m.dlProgress = progressModel.(progress.Model)
		return m, cmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *model) getViewHeight() int {
	h := m.height - 9
	if h < 5 {
		h = 5
	}
	return h
}

func (m *model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit", m.err)
	}

	paneHeight := m.getViewHeight()

	bucketList := []string{headerStyle.Render("BUCKETS")}
	startB := m.offsetBucket
	endB := startB + paneHeight - 1
	if endB > len(m.buckets) {
		endB = len(m.buckets)
	}

	for i := startB; i < endB; i++ {
		label := m.buckets[i]
		s := itemStyle.Render(label)
		if i == m.cursorBucket && m.activePane == paneBuckets {
			s = selectedItemStyle.Render("> " + label)
		}
		bucketList = append(bucketList, s)
	}
	bucketsView := lipgloss.JoinVertical(lipgloss.Left, bucketList...)
	if len(m.buckets) == 0 && !m.loading {
		bucketsView = lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Render("BUCKETS"),
			"No buckets found",
		)
	}

	var objectList []string
	prefixTitle := m.bucket + "/" + m.prefix
	if m.loading {
		prefixTitle += " " + m.spinner.View()
	}
	objectList = append(objectList, headerStyle.Render(prefixTitle))

	startO := m.offsetObject
	endO := startO + (paneHeight - 2)
	if endO > len(m.objects) {
		endO = len(m.objects)
	}

	for i := startO; i < endO; i++ {
		o := m.objects[i]
		var icon string
		if o.IsDir {
			icon = dirStyle.Render("[DIR]")
		} else {
			icon = fileStyle.Render("[FILE]")
		}

		label := icon + " " + o.Name
		if !o.IsDir {
			label += fmt.Sprintf("  %s", formatSize(o.Size))
		}

		s := itemStyle.Render(label)
		if i == m.cursorObject && m.activePane == paneObjects {
			s = selectedItemStyle.Render("> " + label)
		}
		objectList = append(objectList, s)
	}
	objectsView := lipgloss.JoinVertical(lipgloss.Left, objectList...)
	if len(m.objects) == 0 && !m.loading && m.bucket != "" {
		emptyMsg := "Empty bucket/prefix"
		if m.loading {
			emptyMsg = "Loading objects..."
		}
		objectsView = lipgloss.JoinVertical(lipgloss.Left, headerStyle.Render(prefixTitle), emptyMsg)
	}

	leftWidth := 30
	rightWidth := m.width - leftWidth - 6
	if rightWidth < 20 {
		rightWidth = 20
	}

	leftStyle := paneStyle.Width(leftWidth).Height(paneHeight).MaxHeight(paneHeight)
	if m.activePane == paneBuckets {
		leftStyle = activePaneStyle.Width(leftWidth).Height(paneHeight).MaxHeight(paneHeight)
	}

	rightStyle := paneStyle.Width(rightWidth).Height(paneHeight).MaxHeight(paneHeight)
	if m.activePane == paneObjects {
		rightStyle = activePaneStyle.Width(rightWidth).Height(paneHeight).MaxHeight(paneHeight)
	}

	panes := lipgloss.JoinHorizontal(lipgloss.Top,
		leftStyle.Render(bucketsView),
		rightStyle.Render(objectsView),
	)

	var bottomView string

	colWidth := (m.width - 4) / 3
	if colWidth < 25 {
		colWidth = 25
	}

	var progressContent string
	if m.downloading {
		progressContent = fmt.Sprintf("Downloading: %s\n%s", m.dlName, m.dlProgress.View())
	} else if m.dlStatus != "" {
		progressContent = m.dlStatus
	} else {
		progressContent = "No active transfers"
	}
	progressCol := bottomPanelStyle.Width(colWidth).Height(5).MaxHeight(5).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Width(colWidth-2).Render("PROGRESS"),
			progressContent,
		),
	)

	historyToShow := m.taskHistory
	if len(historyToShow) > 8 {
		historyToShow = historyToShow[len(historyToShow)-8:]
	}
	var historyContent string
	if len(historyToShow) > 0 {
		historyContent = lipgloss.JoinVertical(lipgloss.Left, historyToShow...)
	} else {
		historyContent = "No history"
	}
	historyCol := bottomPanelStyle.Width(colWidth).Height(5).MaxHeight(5).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Width(colWidth-2).Render("HISTORY"),
			historyContent,
		),
	)

	var metadataContent string
	if m.activePane == paneBuckets && len(m.buckets) > 0 {
		metadataContent = fmt.Sprintf("Bucket: %s", m.buckets[m.cursorBucket])
	} else if m.activePane == paneObjects && len(m.objects) > 0 {
		obj := m.objects[m.cursorObject]
		metadataContent = fmt.Sprintf("Name: %s\nSize: %s\nType: %s",
			obj.Name,
			formatSize(obj.Size),
			map[bool]string{true: "Directory", false: "File"}[obj.IsDir],
		)
		if obj.LastModified != nil {
			metadataContent += fmt.Sprintf("\nModified: %s", *obj.LastModified)
		}
	} else {
		metadataContent = "No selection"
	}
	metadataCol := bottomPanelStyle.Width(colWidth).Height(5).MaxHeight(5).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Width(colWidth-2).Render("METADATA"),
			metadataContent,
		),
	)

	bottomView = lipgloss.JoinHorizontal(lipgloss.Top,
		progressCol,
		historyCol,
		metadataCol,
	)

	helpView := helpStyle.Render(m.help.View(m.keys))

	finalView := lipgloss.JoinVertical(lipgloss.Left,
		panes,
		bottomView,
		helpView,
	)

	if m.overlay == overlayPalette {
		palette := dialogStyle.Render(
			lipgloss.JoinVertical(lipgloss.Center,
				headerStyle.Render("COMMAND PALETTE"),
				"",
				"Available Commands:",
				"• Refresh (r)",
				"• Upload (u) - Use CLI",
				"• Delete (d) - Use CLI",
				"• Copy S3 URI (c)",
				"",
				lipgloss.NewStyle().Foreground(subtleColor).Render("Press Esc to close"),
			),
		)
		return m.placeOverlay(finalView, palette)
	}

	if m.overlay == overlayProperties && m.propEntry != nil {
		props := dialogStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				headerStyle.Render("PROPERTIES: "+m.propEntry.Name),
				"",
				fmt.Sprintf("Size:          %s", formatSize(m.propEntry.Size)),
				fmt.Sprintf("Last Modified: %s", *m.propEntry.LastModified),
				fmt.Sprintf("Storage Class: %s", m.propEntry.StorageClass),
				fmt.Sprintf("ETag:          %s", m.propEntry.ETag),
				"",
				lipgloss.NewStyle().Foreground(subtleColor).Render("Press Esc to close"),
			),
		)
		return m.placeOverlay(finalView, props)
	}

	return finalView
}

func (m *model) placeOverlay(base string, overlay string) string {
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)

	topPadding := (m.height - overlayHeight) / 2
	leftPadding := (m.width - overlayWidth) / 2

	if topPadding < 0 {
		topPadding = 0
	}
	if leftPadding < 0 {
		leftPadding = 0
	}

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(subtleColor),
	)
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}

func (m *model) addHistory(msg string) {
	m.taskHistory = append(m.taskHistory, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	if len(m.taskHistory) > 100 {
		m.taskHistory = m.taskHistory[1:]
	}
}

func (m *model) startDownload(obj S3Entry) tea.Cmd {
	key := m.prefix + obj.Name
	m.dlName = obj.Name
	m.downloading = true
	m.dlProgress.SetPercent(0)
	m.dlStatus = ""

	return func() tea.Msg {
		outputPath := filepath.Base(obj.Name)
		err := downloadObject(context.Background(), m.client, m.bucket, key, outputPath, func(p Progress) {
			if m.program != nil {
				m.program.Send(dlProgressMsg(float64(p.DownloadedBytes) / float64(p.TotalBytes)))
			}
		})
		return dlDoneMsg{err: err}
	}
}

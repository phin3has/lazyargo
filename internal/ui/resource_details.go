package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"sigs.k8s.io/yaml"

	"lazyargo/internal/argocd"
)

type resourceDetailsTab int

const (
	resourceTabLive resourceDetailsTab = iota
	resourceTabDesired
)

type resourceDetailsModel struct {
	styles styles
	client argocd.Client

	appName string
	ref     argocd.ResourceRef

	width  int
	height int

	vp viewport.Model

	loading bool
	err     error

	liveManifest    string
	desiredManifest string

	tab        resourceDetailsTab
	showAsJSON bool
}

type resourceDetailsLoadedMsg struct {
	live    string
	desired string
	err     error
}

func newResourceDetailsModel(styles styles, client argocd.Client, appName string, ref argocd.ResourceRef) resourceDetailsModel {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = false

	return resourceDetailsModel{
		styles:  styles,
		client:  client,
		appName: appName,
		ref:     ref,
		vp:      vp,
		loading: true,
		tab:     resourceTabLive,
	}
}

func (m resourceDetailsModel) initCmd() tea.Cmd {
	return func() tea.Msg {
		live, err := m.client.GetResource(context.Background(), m.appName, m.ref)
		if err != nil {
			return resourceDetailsLoadedMsg{err: err}
		}

		manifests, err := m.client.GetManifests(context.Background(), m.appName)
		if err != nil {
			return resourceDetailsLoadedMsg{live: live, err: err}
		}
		desired := findDesiredManifest(manifests, m.ref)
		return resourceDetailsLoadedMsg{live: live, desired: desired, err: nil}
	}
}

func (m *resourceDetailsModel) setSize(w, h int) {
	m.width = w
	m.height = h
	// Header takes ~2 lines.
	innerH := max(1, h-2)
	m.vp.Width = max(1, w)
	m.vp.Height = innerH
	m.vp.SetContent(m.renderBody())
}

func (m resourceDetailsModel) Update(msg tea.Msg) (resourceDetailsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil
	case resourceDetailsLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.liveManifest = msg.live
		m.desiredManifest = msg.desired
		m.vp.SetContent(m.renderBody())
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			// parent handles close
			return m, nil
		case "tab":
			if m.tab == resourceTabLive {
				m.tab = resourceTabDesired
			} else {
				m.tab = resourceTabLive
			}
			m.vp.SetContent(m.renderBody())
			return m, nil
		case "j", "down", "k", "up", "pgdown", "pgup":
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		case "t":
			m.showAsJSON = !m.showAsJSON
			m.vp.SetContent(m.renderBody())
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m resourceDetailsModel) View() string {
	header := fmt.Sprintf("Resource: %s/%s (%s)  [tab=%s]  [t=%s]  esc=close",
		m.ref.Kind,
		m.ref.Name,
		blankIfEmpty(m.ref.Namespace, "cluster"),
		map[resourceDetailsTab]string{resourceTabLive: "Live", resourceTabDesired: "Desired"}[m.tab],
		map[bool]string{false: "yaml", true: "json"}[m.showAsJSON],
	)

	headStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
	body := m.vp.View()
	return lipgloss.JoinVertical(lipgloss.Top, headStyle.Width(m.width).Render(header), body)
}

func (m resourceDetailsModel) renderBody() string {
	if m.loading {
		return "Loading…"
	}
	if m.err != nil {
		return "Error:\n\n" + m.err.Error()
	}

	var s string
	if m.tab == resourceTabLive {
		s = m.liveManifest
		if strings.TrimSpace(s) == "" {
			s = "(empty live manifest)"
		}
	} else {
		s = m.desiredManifest
		if strings.TrimSpace(s) == "" {
			s = "(desired manifest not found via /manifests)"
		}
	}

	if m.showAsJSON {
		// Best-effort YAML->JSON; if it fails, show original.
		var obj any
		if err := yaml.Unmarshal([]byte(s), &obj); err == nil {
			b, err := json.MarshalIndent(obj, "", "  ")
			if err == nil {
				return string(b)
			}
		}
	}
	return s
}

func findDesiredManifest(manifests []string, ref argocd.ResourceRef) string {
	wantKind := strings.TrimSpace(ref.Kind)
	wantName := strings.TrimSpace(ref.Name)
	wantNS := strings.TrimSpace(ref.Namespace)

	// Best-effort parse manifests.
	for _, m := range manifests {
		var obj map[string]any
		if err := yaml.Unmarshal([]byte(m), &obj); err != nil {
			continue
		}
		kind, _ := obj["kind"].(string)
		if strings.TrimSpace(kind) != wantKind {
			continue
		}
		meta, _ := obj["metadata"].(map[string]any)
		name, _ := meta["name"].(string)
		ns, _ := meta["namespace"].(string)
		if strings.TrimSpace(name) != wantName {
			continue
		}
		if wantNS != "" && strings.TrimSpace(ns) != wantNS {
			continue
		}
		return m
	}
	return ""
}

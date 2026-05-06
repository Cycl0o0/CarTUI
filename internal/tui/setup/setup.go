// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package setup hosts the first-run welcome screen. The user picks how
// CarTUI should fetch map data — online with no key, online with an
// API key, online via a self-hosted instance, or offline — and the
// chosen choice is written back to the config file as a starting point
// they can later edit.
package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Choice identifies one of the offered map-source flavours.
type Choice uint8

// Map-source choices, ordered from "easiest" to "advanced".
const (
	ChoiceUnset Choice = iota
	ChoiceOpenFreeMap
	ChoiceMapbox
	ChoicePMTiles
	ChoiceOverpassPublic
	ChoiceOverpassLocal
	ChoiceOfflinePBF
	ChoiceCancel
)

// Result is the value returned by [Run] after the user has confirmed.
type Result struct {
	Choice Choice

	// Filled when the user picked a source that needs an extra value.
	MapboxToken      string
	PMTilesURL       string
	OverpassURL      string
	OfflinePBFRegion string
}

// region describes one offline-PBF preset shown in the sub-list.
type region struct {
	Name     string
	URL      string // Geofabrik PBF URL
	SizeMB   int    // download size
	RAMMB    int    // approximate runtime memory after parse+index
	Severity severity
	Note     string
}

type severity uint8

const (
	sevOK severity = iota
	sevModerate
	sevHeavy
	sevExtreme
)

// regionPresets is the menu of offline-PBF candidates. Sizes are
// rounded; warning copy explains the trade-off.
var regionPresets = []region{
	{
		Name:   "Aquitaine (région)",
		URL:    "https://download.geofabrik.de/europe/france/aquitaine-latest.osm.pbf",
		SizeMB: 150, RAMMB: 80, Severity: sevOK,
		Note: "rapide, parfait pour tester ; couvre le sud-ouest français",
	},
	{
		Name:   "France",
		URL:    "https://download.geofabrik.de/europe/france-latest.osm.pbf",
		SizeMB: 5000, RAMMB: 1500, Severity: sevModerate,
		Note: "couvre toute la France ; init ~5 min, ~1.5 GB RAM",
	},
	{
		Name:   "Europe",
		URL:    "https://download.geofabrik.de/europe-latest.osm.pbf",
		SizeMB: 28000, RAMMB: 7000, Severity: sevHeavy,
		Note: "init ~30 min, ~7 GB RAM — pour usage intensif uniquement",
	},
	{
		Name:   "Monde entier (planet)",
		URL:    "https://planet.openstreetmap.org/pbf/planet-latest.osm.pbf",
		SizeMB: 80000, RAMMB: 30000, Severity: sevExtreme,
		Note: "init ~6 h, 30+ GB RAM — uniquement sur serveur dédié",
	},
}

// regionItem implements bubbles list.Item.
type regionItem struct{ R region }

func (r regionItem) FilterValue() string { return r.R.Name }
func (r regionItem) Title() string {
	icon := ""
	switch r.R.Severity {
	case sevOK:
		icon = "✓ "
	case sevModerate:
		icon = "  "
	case sevHeavy:
		icon = "⚠ "
	case sevExtreme:
		icon = "⚠⚠⚠ "
	}
	return fmt.Sprintf("%s%s — %s · %s",
		icon, r.R.Name, formatSize(r.R.SizeMB), formatRAM(r.R.RAMMB))
}
func (r regionItem) Description() string { return r.R.Note }

// choiceItem represents one top-level option in the welcome list.
type choiceItem struct {
	choice Choice
	title  string
	desc   string
	warn   string
}

func (c choiceItem) FilterValue() string { return c.title }
func (c choiceItem) Title() string {
	t := c.title
	if c.warn != "" {
		t += " " + c.warn
	}
	return t
}
func (c choiceItem) Description() string { return c.desc }

var topChoices = []choiceItem{
	{
		choice: ChoiceOpenFreeMap,
		title:  "OpenFreeMap (recommandé)",
		desc:   "Tuiles vector OSM gratuites via Cloudflare. Aucune clé. Toujours en ligne.",
	},
	{
		choice: ChoiceMapbox,
		title:  "Mapbox Streets v8",
		desc:   "Tuiles plus riches mais nécessite une clé API gratuite (50k req/mois).",
		warn:   "🔑",
	},
	{
		choice: ChoicePMTiles,
		title:  "Protomaps PMTiles (planet)",
		desc:   "Vector tiles via HTTP Range, archive globale, ~135 GB en ligne (rien à télécharger).",
	},
	{
		choice: ChoiceOverpassPublic,
		title:  "Overpass API publique",
		desc:   "Multi-mirror fallback (private.coffee, overpass-api.de…). Gratuit mais flaky.",
	},
	{
		choice: ChoiceOverpassLocal,
		title:  "Overpass auto-hébergée",
		desc:   "Utilise une instance Overpass que tu fais tourner toi-même (Docker, etc.).",
	},
	{
		choice: ChoiceOfflinePBF,
		title:  "Hors-ligne (base OSM locale)",
		desc:   "Télécharge un .osm.pbf et l'index spatial reste en RAM. Latence imbattable.",
		warn:   "⚠ avancé",
	},
	{
		choice: ChoiceCancel,
		title:  "Annuler",
		desc:   "Garde la config actuelle.",
	},
}

// stage is the internal welcome state machine.
type stage uint8

const (
	stageMain stage = iota
	stageMapboxToken
	stagePMTilesURL
	stageOverpassURL
	stageOfflineRegion
	stageOfflineConfirm
	stageDone
)

// Model is the Bubble Tea model exposed to the runtime.
type Model struct {
	stage   stage
	width   int
	height  int
	main    list.Model
	regions list.Model
	input   textinput.Model

	pickedChoice Choice
	pickedRegion *region

	result Result
	done   bool
}

// New builds a welcome screen.
func New() Model {
	mainItems := make([]list.Item, len(topChoices))
	for i, c := range topChoices {
		mainItems[i] = c
	}
	mainList := list.New(mainItems, list.NewDefaultDelegate(), 60, 16)
	mainList.Title = "Bienvenue dans CarTUI"
	mainList.SetShowStatusBar(false)
	mainList.SetShowHelp(false)
	mainList.SetFilteringEnabled(false)

	regionItems := make([]list.Item, len(regionPresets))
	for i, r := range regionPresets {
		regionItems[i] = regionItem{R: r}
	}
	regionList := list.New(regionItems, list.NewDefaultDelegate(), 70, 12)
	regionList.Title = "Choisis la zone à charger localement"
	regionList.SetShowStatusBar(false)
	regionList.SetShowHelp(false)
	regionList.SetFilteringEnabled(false)

	in := textinput.New()
	in.Prompt = "> "
	in.CharLimit = 1024

	return Model{
		stage:   stageMain,
		main:    mainList,
		regions: regionList,
		input:   in,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Done reports whether the user has confirmed (or cancelled).
func (m Model) Done() bool { return m.done }

// Result returns the final user choice (only meaningful after Done()).
func (m Model) Result() Result { return m.result }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.main.SetSize(min(80, msg.Width-4), msg.Height-10)
		m.regions.SetSize(min(80, msg.Width-4), msg.Height-12)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.stage == stageMain {
				m.result = Result{Choice: ChoiceCancel}
				m.done = true
				return m, tea.Quit
			}
			m.stage = stageMain
			m.input.Blur()
			m.input.SetValue("")
			return m, nil
		case "enter":
			return m.handleEnter()
		}
	}

	var cmd tea.Cmd
	switch m.stage {
	case stageMain:
		m.main, cmd = m.main.Update(msg)
	case stageOfflineRegion:
		m.regions, cmd = m.regions.Update(msg)
	case stageMapboxToken, stagePMTilesURL, stageOverpassURL:
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

// handleEnter advances the state machine when the user presses Enter.
func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.stage {
	case stageMain:
		it, ok := m.main.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.pickedChoice = it.choice
		switch it.choice {
		case ChoiceOpenFreeMap:
			m.result = Result{Choice: ChoiceOpenFreeMap}
			m.done = true
			return m, tea.Quit
		case ChoiceCancel:
			m.result = Result{Choice: ChoiceCancel}
			m.done = true
			return m, tea.Quit
		case ChoiceMapbox:
			m.stage = stageMapboxToken
			m.input.Placeholder = "pk.eyJ1Ijoi…"
			m.input.SetValue("")
			cmd := m.input.Focus()
			return m, cmd
		case ChoicePMTiles:
			m.stage = stagePMTilesURL
			m.input.Placeholder = "https://build.protomaps.com/20260505.pmtiles ou /chemin/local.pmtiles"
			m.input.SetValue("")
			cmd := m.input.Focus()
			return m, cmd
		case ChoiceOverpassPublic:
			// Empty url -> default rotation (private.coffee + mirrors).
			m.result = Result{Choice: ChoiceOverpassPublic}
			m.done = true
			return m, tea.Quit
		case ChoiceOverpassLocal:
			m.stage = stageOverpassURL
			m.input.Placeholder = "http://localhost:12345/api/interpreter"
			m.input.SetValue("")
			cmd := m.input.Focus()
			return m, cmd
		case ChoiceOfflinePBF:
			m.stage = stageOfflineRegion
			return m, nil
		}

	case stageMapboxToken:
		v := strings.TrimSpace(m.input.Value())
		if v == "" {
			return m, nil
		}
		m.result = Result{Choice: ChoiceMapbox, MapboxToken: v}
		m.done = true
		return m, tea.Quit

	case stagePMTilesURL:
		v := strings.TrimSpace(m.input.Value())
		if v == "" {
			return m, nil
		}
		m.result = Result{Choice: ChoicePMTiles, PMTilesURL: v}
		m.done = true
		return m, tea.Quit

	case stageOverpassURL:
		v := strings.TrimSpace(m.input.Value())
		if v == "" {
			return m, nil
		}
		m.result = Result{Choice: ChoiceOverpassLocal, OverpassURL: v}
		m.done = true
		return m, tea.Quit

	case stageOfflineRegion:
		it, ok := m.regions.SelectedItem().(regionItem)
		if !ok {
			return m, nil
		}
		r := it.R
		m.pickedRegion = &r
		m.stage = stageOfflineConfirm
		return m, nil

	case stageOfflineConfirm:
		if m.pickedRegion == nil {
			m.stage = stageOfflineRegion
			return m, nil
		}
		m.result = Result{Choice: ChoiceOfflinePBF, OfflinePBFRegion: m.pickedRegion.Name}
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFA500")).
		Render("CarTUI · première configuration") + "\n"

	footer := lipgloss.NewStyle().Faint(true).Render(
		"↑↓ naviguer · Enter confirmer · Esc retour/quitter") + "\n"

	switch m.stage {
	case stageMain:
		return header + "\n" + m.main.View() + "\n" + footer
	case stageMapboxToken:
		return header +
			"\nColle ton token Mapbox public :\n  " + m.input.View() +
			"\n\n" + lipgloss.NewStyle().Faint(true).Render(
			"Récupère-le sur https://account.mapbox.com/access-tokens/.\n"+
				"Free tier : 50k requêtes/mois.") + "\n\n" + footer
	case stagePMTilesURL:
		return header +
			"\nURL ou chemin du fichier PMTiles :\n  " + m.input.View() +
			"\n\n" + lipgloss.NewStyle().Faint(true).Render(
			"Le planet build est ici : https://build.protomaps.com/{date}.pmtiles\n"+
				"Pour un extract local, génère-le avec `pmtiles extract`.") + "\n\n" + footer
	case stageOverpassURL:
		return header +
			"\nURL de l'instance Overpass à utiliser :\n  " + m.input.View() +
			"\n\n" + lipgloss.NewStyle().Faint(true).Render(
			"Plusieurs URLs séparées par virgule = rotation/fallback automatique.") +
			"\n\n" + footer
	case stageOfflineRegion:
		warning := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF8888")).
			Render("⚠ La zone choisie sera téléchargée puis indexée en RAM.")
		return header + "\n" + warning + "\n\n" + m.regions.View() + "\n" + footer
	case stageOfflineConfirm:
		if m.pickedRegion == nil {
			return header + "\n(no region selected)\n" + footer
		}
		r := *m.pickedRegion
		title := lipgloss.NewStyle().Bold(true).Render(r.Name)
		warn := severityBlock(r)
		body := strings.Join([]string{
			"",
			title,
			"",
			fmt.Sprintf("Téléchargement      : %s", formatSize(r.SizeMB)),
			fmt.Sprintf("RAM après indexation : %s", formatRAM(r.RAMMB)),
			fmt.Sprintf("Source              : %s", r.URL),
			"",
			warn,
			"",
			lipgloss.NewStyle().Bold(true).Render("Enter pour confirmer · Esc pour revenir"),
		}, "\n")
		return header + body + "\n\n" + footer
	}
	return header + "?\n" + footer
}

// severityBlock returns a coloured warning block for a given region's
// severity tier.
func severityBlock(r region) string {
	style := lipgloss.NewStyle().Padding(0, 1)
	switch r.Severity {
	case sevOK:
		return style.
			Background(lipgloss.Color("#1c4e1c")).
			Foreground(lipgloss.Color("#cfeacc")).
			Render("✓ Choix sûr — démarre vite, peu de RAM.")
	case sevModerate:
		return style.
			Background(lipgloss.Color("#3d3a1f")).
			Foreground(lipgloss.Color("#fff5cf")).
			Render("ℹ Init ~5 min · " + formatRAM(r.RAMMB) +
				" RAM constante. Convient à un PC moderne.")
	case sevHeavy:
		return style.
			Background(lipgloss.Color("#5a3000")).
			Foreground(lipgloss.Color("#ffe1b3")).
			Render("⚠ HEAVY — assure-toi d'avoir " +
				formatRAM(r.RAMMB) + " RAM dispo et 30+ min devant toi.")
	case sevExtreme:
		return style.
			Background(lipgloss.Color("#5e0010")).
			Foreground(lipgloss.Color("#ffd0d0")).
			Render("⚠⚠⚠ EXTRÊME — déconseillé sur un poste de travail. " +
				formatRAM(r.RAMMB) + " RAM, plusieurs heures d'init, " +
				formatSize(r.SizeMB) + " disque.")
	}
	return ""
}

// formatSize renders an MB count as "XXX MB" or "X.X GB".
func formatSize(mb int) string {
	if mb < 1024 {
		return fmt.Sprintf("%d MB", mb)
	}
	return fmt.Sprintf("%.1f GB", float64(mb)/1024)
}

// formatRAM is the same as formatSize but with a "RAM" suffix.
func formatRAM(mb int) string {
	return formatSize(mb) + " RAM"
}

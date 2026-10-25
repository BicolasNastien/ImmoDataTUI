package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Navigation par la carte
// ---------------------------------------------------------------------------
//
// Trois échelons successifs, chacun rendu par le même composant de carte :
//
//	régions  ──▶  départements de la région  ──▶  communes du département
//	                        │                              │
//	                        └── téléchargement des         └── extraction puis
//	                            millésimes du dép.             écrans de stats
//
// À l'échelon des communes, la France et ses cartouches d'outre-mer
// disparaissent : on ne montre que le département, découpé en communes.

type niveau int

const (
	nivRegions niveau = iota
	nivDepartements
	nivCommunes
)

const (
	largeurListe = 30
	entetes      = 3 // lignes au-dessus de la carte : marge, titre, marge
	margeGauche  = 2
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// msgZones porte le résultat d'un chargement de contours.
type msgZones struct {
	niveau niveau
	zones  []Zone
	err    error
}

// msgTelechargement porte le résultat d'un millésime. Un message par année :
// Bubble Tea exécute les commandes d'un lot en parallèle, la progression
// s'affiche donc au fil de l'eau sans aucune synchronisation à écrire.
type msgTelechargement ResultatTelechargement

// msgExtraction porte les millésimes disponibles pour une commune.
type msgExtraction struct {
	commune Zone
	annees  []string
	err     error
}

// ---------------------------------------------------------------------------
// Commandes
// ---------------------------------------------------------------------------

func cmdRegions() tea.Cmd {
	return func() tea.Msg {
		z, err := Regions()
		return msgZones{niveau: nivRegions, zones: z, err: err}
	}
}

func cmdDepartements(codeRegion string) tea.Cmd {
	return func() tea.Msg {
		z, err := DepartementsDe(codeRegion)
		return msgZones{niveau: nivDepartements, zones: z, err: err}
	}
}

func cmdCommunes(codeDep string) tea.Cmd {
	return func() tea.Msg {
		z, err := CommunesDe(codeDep)
		return msgZones{niveau: nivCommunes, zones: z, err: err}
	}
}

// cmdTelecharger produit une commande par millésime. La closure capture le
// contexte : une pression sur échap l'annule et toutes les requêtes en vol
// s'arrêtent.
func cmdTelecharger(ctx context.Context, annee, dep string, forcer bool) tea.Cmd {
	return func() tea.Msg {
		return msgTelechargement(TelechargerArchive(ctx, annee, dep, forcer))
	}
}

func cmdExtraire(racine, dep string, commune Zone) tea.Cmd {
	return func() tea.Msg {
		annees, err := ExtraireCommune(racine, dep, commune.Code, commune.Nom)
		return msgExtraction{commune: commune, annees: annees, err: err}
	}
}

// ---------------------------------------------------------------------------
// État de la carte dans le modèle
// ---------------------------------------------------------------------------

type etatCarte struct {
	niveau  niveau
	zones   []Zone
	dep     string // à l'échelon des communes : le département dessiné
	carte   *Carte
	sel     int
	haut    int // premier rang visible de la liste (rang dans visibles)
	marques map[string]bool

	filtre       string // réduit la liste des communes, voir filtre.go
	saisitFiltre bool   // le clavier est au filtre
}

// construire (re)projette les zones sur la place disponible. Appelé au
// changement d'échelon et à chaque redimensionnement du terminal.
func (e *etatCarte) construire(largeur, hauteur int) {
	l := largeur - largeurListe - margeGauche - 2
	h := hauteur - entetes - 3
	if l < 20 {
		l = 20
	}
	if h < 6 {
		h = 6
	}
	if len(e.zones) == 0 {
		e.carte = nil
		return
	}
	e.carte = NouvelleCarte(e.zones, l, h, e.niveau == nivRegions, e.marques)
}

func (e *etatCarte) selection() (Zone, bool) {
	if e.sel < 0 || e.sel >= len(e.zones) {
		return Zone{}, false
	}
	return e.zones[e.sel], true
}

// suivre garde la sélection visible dans la liste latérale. On raisonne en
// rang dans la liste affichée, qui n'est l'indice de la zone que sans filtre.
func (e *etatCarte) suivre(hauteurListe int) {
	p := position(e.visibles(), e.sel)
	if p < 0 {
		return // sélection hors de la liste filtrée : rien à faire défiler
	}
	if p < e.haut {
		e.haut = p
	}
	if p >= e.haut+hauteurListe {
		e.haut = p - hauteurListe + 1
	}
	if e.haut < 0 {
		e.haut = 0
	}
}

// ---------------------------------------------------------------------------
// Écran de téléchargement
// ---------------------------------------------------------------------------

type etatTelechargement struct {
	dep      Zone
	annees   []string
	resultat map[string]ResultatTelechargement
	annuler  context.CancelFunc
}

func (t *etatTelechargement) fini() bool { return len(t.resultat) >= len(t.annees) }

func (t *etatTelechargement) reussi() bool {
	for _, r := range t.resultat {
		if r.Err == nil && !r.Absent {
			return true
		}
	}
	return false
}

func (t *etatTelechargement) erreurs() []error {
	var out []error
	for _, r := range t.resultat {
		if r.Err != nil {
			out = append(out, r.Err)
		}
	}
	return out
}

// vue rend la progression, une ligne par millésime.
func (t etatTelechargement) vue(sp spinner.Model) string {
	var b strings.Builder
	b.WriteString("\n  " + styleTitre.Render("Téléchargement — "+t.dep.Nom+" ("+t.dep.Code+")") + "\n\n")

	for _, a := range t.annees {
		r, connu := t.resultat[a]
		switch {
		case !connu:
			b.WriteString(fmt.Sprintf("   %s  %s\n", sp.View(), a))
		case r.Err != nil:
			b.WriteString(fmt.Sprintf("   %s  %s  %s\n", styleWarn.Render("✗"), a,
				styleWarn.Render(r.Err.Error())))
		case r.Absent:
			b.WriteString(fmt.Sprintf("   %s  %s  %s\n", styleDim.Render("–"), a,
				styleDim.Render("millésime non publié")))
		case r.DejaLa:
			b.WriteString(fmt.Sprintf("   %s  %s  %s\n", styleOK.Render("✓"), a,
				styleDim.Render("déjà en cache · "+octets(r.Octets)+provenance(r))))
		default:
			b.WriteString(fmt.Sprintf("   %s  %s  %s\n", styleOK.Render("✓"), a,
				styleDim.Render("téléchargé · "+octets(r.Octets)+provenance(r))))
		}
	}

	b.WriteString("\n  " + styleDim.Render(
		"Les millésimes sont récupérés en parallèle, trois requêtes au plus à la fois.") + "\n")
	b.WriteString("  " + styleAide.Render("esc annuler · q quitter") + "\n")
	return b.String()
}

// provenance signale un millésime tiré du fichier brut : le seul qui prenne
// plusieurs dizaines de Mo à télécharger, et le seul sans coordonnées.
func provenance(r ResultatTelechargement) string {
	if r.Provisoire {
		return " · fichier brut DGFiP, en attendant geo-dvf"
	}
	return ""
}

func octets(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f Mo", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f Ko", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d o", n)
	}
}

// ---------------------------------------------------------------------------
// Rendu de l'écran carte
// ---------------------------------------------------------------------------

var (
	styleListeSel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(colAccent)
	styleMarque   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

func (m model) vueCarte() string {
	e := m.carte
	var b strings.Builder

	b.WriteString("\n  " + styleTitre.Render(m.titreCarte()) + "\n\n")

	if e.carte == nil {
		b.WriteString("  " + styleWarn.Render("Aucun contour à afficher.") + "\n")
		return b.String()
	}

	// Carte à gauche, liste des zones à droite : nommer chaque zone sur la
	// carte est impossible en terminal, la liste s'en charge et la sélection
	// est mise en évidence des deux côtés.
	var lignes []string
	for _, l := range strings.Split(e.carte.Rendu(e.sel), "\n") {
		lignes = append(lignes, strings.Repeat(" ", margeGauche)+l)
	}
	gauche := strings.Join(lignes, "\n")
	droite := m.vueListeZones(e.carte.H)

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, gauche, "  ", droite))
	b.WriteString("\n\n  " + styleAide.Render(m.aideCarte()))
	return b.String()
}

func (m model) titreCarte() string {
	switch m.carte.niveau {
	case nivRegions:
		return "DVF — quelle région ?"
	case nivDepartements:
		return m.region.Nom + " — quel département ?"
	default:
		titre := m.dep.Nom + " (" + m.dep.Code + ") — quelle commune ?"
		if m.carte.filtreActif() {
			curseur := ""
			if m.carte.saisitFiltre {
				curseur = "▏"
			}
			titre += "   / " + m.carte.filtre + curseur + "  " +
				styleDim.Render(pluriel(len(m.carte.visibles()), "commune"))
		}
		return titre
	}
}

func (m model) aideCarte() string {
	if m.carte.saisitFiltre {
		return "tape le nom · ↑/↓ choisir · ↵ ouvrir · esc effacer le filtre"
	}
	if m.carte.niveau == nivCommunes {
		base := "↑/↓ choisir · ↵ ouvrir · / filtrer · A adresse · a en vente · esc retour · q quitter"
		if m.carte.filtre != "" {
			base = "↑/↓ choisir · ↵ ouvrir · / modifier le filtre · esc effacer le filtre · q quitter"
		}
		return base
	}
	base := "↑/↓ choisir · clic sélectionner · ↵ ouvrir · / adresse · a biens en vente · q quitter"
	if m.carte.niveau != nivRegions {
		base = "↑/↓ choisir · clic sélectionner · ↵ ouvrir · / adresse · a en vente · esc retour · q quitter"
	}
	if m.carte.niveau == nivDepartements {
		base += " · " + styleMarque.Render("vert = déjà téléchargé")
	}
	if len(m.villes) > 0 && m.carte.niveau == nivRegions {
		base += " · l collection locale"
	}
	return base
}

func (m model) vueListeZones(hauteur int) string {
	e := m.carte
	var b strings.Builder

	vis := e.visibles()
	fin := e.haut + hauteur
	if fin > len(vis) {
		fin = len(vis)
	}
	if len(vis) == 0 {
		b.WriteString(styleDim.Render(" aucune commune ne correspond"))
	}

	for _, i := range vis[min(e.haut, fin):fin] {
		z := e.zones[i]
		nom := z.Nom
		if e.marques[z.Code] {
			nom = "● " + nom
		} else {
			nom = "  " + nom
		}
		if len([]rune(nom)) > largeurListe-2 {
			nom = string([]rune(nom)[:largeurListe-3]) + "…"
		}
		ligne := fmt.Sprintf(" %-*s", largeurListe-2, nom)
		if i == e.sel {
			b.WriteString(styleListeSel.Render(ligne))
		} else if e.marques[z.Code] {
			b.WriteString(styleMarque.Render(ligne))
		} else {
			b.WriteString(ligne)
		}
		b.WriteString("\n")
	}

	if len(vis) > hauteur {
		b.WriteString(styleDim.Render(fmt.Sprintf(" %d/%d", position(vis, e.sel)+1, len(vis))))
	}
	return strings.TrimRight(b.String(), "\n")
}

// ---------------------------------------------------------------------------
// Chemins de données
// ---------------------------------------------------------------------------

// dossierCommune est l'emplacement où sont extraits les millésimes d'une
// commune, dans l'arborescence que le programme sait déjà lire.
func (m model) dossierCommune(annee string) string {
	return filepath.Join(m.racine, NormaliserNom(m.commune.Nom), annee)
}

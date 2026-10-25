package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	colAccent = lipgloss.Color("39")  // bleu
	colDim    = lipgloss.Color("245") // gris
	colWarn   = lipgloss.Color("214") // orange
	colOK     = lipgloss.Color("42")  // vert

	styleTitre = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleAide  = lipgloss.NewStyle().Foreground(colDim)
	styleDim   = lipgloss.NewStyle().Foreground(colDim)
	styleWarn  = lipgloss.NewStyle().Foreground(colWarn)
	styleOK    = lipgloss.NewStyle().Foreground(colOK)
	styleSel   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(colAccent)
	styleTab   = lipgloss.NewStyle().Padding(0, 2).Foreground(colDim)
	styleTabOn = lipgloss.NewStyle().Padding(0, 2).Bold(true).
			Foreground(lipgloss.Color("231")).Background(colAccent)
	styleSection = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
)

// ---------------------------------------------------------------------------
// Formatage
// ---------------------------------------------------------------------------

// euros formate un montant à la française : 485 000 €
func euros(v float64) string {
	if v <= 0 {
		return "–"
	}
	s := fmt.Sprintf("%.0f", v)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, c)
	}
	return string(out) + " €"
}

func eurosM2(v float64) string {
	if v <= 0 {
		return "–"
	}
	return strings.TrimSuffix(euros(v), " €") + " €/m²"
}

// pluriel accorde un décompte : « 1 fichier », « 5 fichiers ».
func pluriel(n int, mot string) string {
	return fmt.Sprintf("%d %s%s", n, mot, marqueS(n))
}

func marqueS(n int) string {
	if n > 1 {
		return "s"
	}
	return ""
}

func m2(v float64) string {
	if v <= 0 {
		return "–"
	}
	return fmt.Sprintf("%.0f m²", v)
}

// ---------------------------------------------------------------------------
// Découverte de l'arborescence
// ---------------------------------------------------------------------------

// Ville représente un sous-dossier racine contenant des dossiers année.
type Ville struct {
	Nom    string
	Chemin string
	Annees []Annee
}

// Annee représente un sous-dossier contenant les CSV d'une année.
type Annee struct {
	Nom    string
	Chemin string
}

// ScannerRacine parcourt le dossier racine : un sous-dossier = une ville,
// un sous-sous-dossier = une année.
func ScannerRacine(racine string) ([]Ville, error) {
	entries, err := os.ReadDir(racine)
	if err != nil {
		return nil, err
	}

	var villes []Ville
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		cheminVille := filepath.Join(racine, e.Name())
		sous, err := os.ReadDir(cheminVille)
		if err != nil {
			continue
		}
		var annees []Annee
		for _, s := range sous {
			if !s.IsDir() || strings.HasPrefix(s.Name(), ".") {
				continue
			}
			annees = append(annees, Annee{Nom: s.Name(), Chemin: filepath.Join(cheminVille, s.Name())})
		}
		if len(annees) == 0 {
			continue
		}
		sort.Slice(annees, func(i, j int) bool { return annees[i].Nom > annees[j].Nom })
		villes = append(villes, Ville{Nom: e.Name(), Chemin: cheminVille, Annees: annees})
	}
	sort.Slice(villes, func(i, j int) bool { return villes[i].Nom < villes[j].Nom })
	return villes, nil
}

// ---------------------------------------------------------------------------
// Modèle Bubble Tea
// ---------------------------------------------------------------------------

type ecran int

const (
	ecranCarte ecran = iota
	ecranTelechargement
	ecranAttente
	ecranVilles
	ecranAnnees
	ecranStats
	ecranVente
	ecranRecherche
	ecranAnnonces  // formulaire de critères des biens en vente
	ecranZones     // choix de la zone de recherche
	ecranResultats // tableau des annonces
)

// ongletsStandard : les onglets d'une commune. Ils découpent le marché par type
// de bien — la période, elle, se choisit un écran plus tôt, dans la liste des
// millésimes. L'onglet « Adresse » vient s'ajouter en tête quand on est arrivé
// par une recherche d'adresse. Le dernier sort de DVF : il confronte la période
// aux biens en vente aujourd'hui.
var ongletsStandard = []string{"Résumé", "Maisons", "Appartements", "Terrains", ongletMarche}

type model struct {
	racine string
	villes []Ville // collection déposée à la main, toujours consultable

	ecran  ecran
	carte  etatCarte
	tele   etatTelechargement
	region Zone
	dep    Zone

	commune      Zone
	annees       []string
	sourceLocale bool // vrai quand on explore la collection locale

	attente string
	alerte  string
	spinner spinner.Model

	curVille  int
	curAnnee  int
	curOnglet int
	data      Dataset
	erreur    error

	// Tableau des ventes de l'onglet courant.
	// Recherche d'adresse.
	adresse        *Adresse // adresse ouverte : ajoute l'onglet du même nom
	saisie         textinput.Model
	resultats      []Adresse
	curResultat    int
	requeteFaite   string
	chercheEnCours bool

	onglets  []string
	agrege   bool      // le jeu affiché fusionne tous les millésimes
	parAnnee []Dataset // un jeu par millésime, du plus ancien au plus récent
	total    Dataset   // tous les millésimes fusionnés

	noteVue      string // retour de la dernière ouverture dans le navigateur
	curVente     int
	tri          Tri
	ligneSel     int // ligne du contenu où se trouve la vente sélectionnée
	ligneTableau int // ligne du contenu où commence le tableau (-1 : aucun)

	// Biens en vente. L'état est distinct de celui de DVF : ce sont deux jeux
	// de données qui ne se mélangent pas, et les partager obligerait à tester
	// partout de quoi on parle.
	formAnn        formAnnonces
	critAnn        Criteres
	triAnn         TriAnnonces
	saisieZone     textinput.Model
	requeteZone    string
	chercheZone    bool
	zonesTrouvees  []ZoneAnnonce
	curZoneTrouvee int
	annonces       []Annonce
	curAnnonce     int
	totalAnn       int
	erreursAnn     map[string]error
	chercheAnn     bool
	annulerAnn     context.CancelFunc

	// Les annonces de la commune affichée, pour l'onglet « En vente ».
	marche         etatMarche
	curMarche      int   // rang de l'annonce sélectionnée, voir lignesMarche
	lignesAnnonces []int // ligne du contenu de chaque annonce, pour la souris

	vp      viewport.Model
	largeur int
	hauteur int
	pretVP  bool
}

func NouveauModele(racine string, villes []Ville) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colAccent)

	return model{
		racine:  racine,
		villes:  villes,
		onglets: ongletsStandard,
		saisie:  nouvelleSaisie(),
		formAnn: nouveauFormAnnonces(),
		// Le tri par prix croissant est le point de départ : c'est la question
		// qu'on se pose en ouvrant un site d'annonces.
		triAnn:     TriAnnonces{Critere: TriAPrix},
		saisieZone: nouvelleSaisieZone(),
		ecran:      ecranAttente,
		attente:    "Chargement des contours…",
		spinner:    sp,
	}
}

func (m model) Init() tea.Cmd { return tea.Batch(m.spinner.Tick, cmdRegions()) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.largeur, m.hauteur = msg.Width, msg.Height
		hautVP := msg.Height - 7
		if hautVP < 5 {
			hautVP = 5
		}
		if !m.pretVP {
			m.vp = viewport.New(msg.Width, hautVP)
			m.pretVP = true
		} else {
			m.vp.Width, m.vp.Height = msg.Width, hautVP
		}
		m.carte.construire(m.largeur, m.hauteur)
		switch m.ecran {
		case ecranStats:
			m.majStats()
		case ecranVente:
			m.vp.SetContent(m.contenuVente())
		case ecranResultats:
			m.vp.SetContent(m.contenuAnnonces())
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case msgZones:
		return m.appliquerZones(msg)

	case msgTelechargement:
		return m.appliquerTelechargement(ResultatTelechargement(msg))

	case msgExtraction:
		return m.appliquerExtraction(msg)

	case msgAdresses:
		return m.appliquerAdresses(msg)

	case msgNavigateur:
		return m.appliquerNavigateur(msg)

	case msgZonesAnnonces:
		return m.appliquerZonesAnnonces(msg)

	case msgAnnonces:
		return m.appliquerAnnonces(msg)

	case msgMarche:
		return m.appliquerMarche(msg)

	case tea.MouseMsg:
		return m.souris(msg)

	case tea.KeyMsg:
		return m.touche(msg)
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Réception des messages
// ---------------------------------------------------------------------------

func (m model) appliquerZones(msg msgZones) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.alerte = msg.err.Error()
		if len(m.carte.zones) == 0 {
			m.erreur = msg.err
		}
		m.ecran = ecranCarte
		return m, nil
	}

	marques := DepartementsTelecharges()
	if msg.niveau == nivRegions {
		marques = RegionsAvecDonnees(marques)
	}
	if msg.niveau == nivCommunes {
		marques = map[string]bool{}
	}

	m.carte = etatCarte{niveau: msg.niveau, zones: msg.zones, marques: marques}
	if msg.niveau == nivCommunes {
		m.carte.dep = m.dep.Code
	}
	m.carte.construire(m.largeur, m.hauteur)
	m.ecran = ecranCarte
	m.alerte = ""
	return m, nil
}

func (m model) appliquerTelechargement(r ResultatTelechargement) (tea.Model, tea.Cmd) {
	// Un résultat peut arriver après qu'on a quitté l'écran : il ne doit alors
	// plus rien déclencher.
	if m.ecran != ecranTelechargement || m.tele.resultat == nil || r.Dep != m.tele.dep.Code {
		return m, nil
	}
	m.tele.resultat[r.Annee] = r
	if !m.tele.fini() {
		return m, nil
	}

	if m.tele.annuler != nil {
		m.tele.annuler()
	}
	if !m.tele.reussi() {
		m.alerte = "Aucun millésime récupéré pour le " + m.tele.dep.Code
		if errs := m.tele.erreurs(); len(errs) > 0 {
			m.alerte = errs[0].Error()
		}
		return m, nil
	}

	m.ecran = ecranAttente
	if m.adresse != nil {
		m.attente = "Extraction de " + m.commune.Nom + "…"
		return m, tea.Batch(m.spinner.Tick, cmdExtraire(m.racine, m.tele.dep.Code, m.commune))
	}
	m.attente = "Contours des communes du " + m.tele.dep.Code + "…"
	return m, tea.Batch(m.spinner.Tick, cmdCommunes(m.tele.dep.Code))
}

func (m model) appliquerExtraction(msg msgExtraction) (tea.Model, tea.Cmd) {
	if msg.err != nil || len(msg.annees) == 0 {
		m.alerte = "Aucune transaction pour " + msg.commune.Nom + " sur les millésimes disponibles."
		if msg.err != nil {
			m.alerte = msg.err.Error()
		}
		m.oublierAdresse()
		m.ecran = ecranCarte
		return m, nil
	}

	m.commune = msg.commune
	m.annees = msg.annees
	m.sourceLocale = false
	m.curAnnee = 0
	m.alerte = ""

	// Arrivé par la carte : on choisit un millésime. Arrivé par une recherche
	// d'adresse : le millésime n'a pas d'intérêt, c'est l'agrégat qu'on veut.
	if m.adresse == nil {
		m.ecran = ecranAnnees
		return m, nil
	}
	return m.ouvrirPageAdresse()
}

// ouvrirPageAdresse charge tous les millésimes et affiche l'onglet Adresse.
func (m model) ouvrirPageAdresse() (tea.Model, tea.Cmd) {
	// Le curseur de la liste des millésimes est laissé sur « toutes les
	// années » : revenir en arrière depuis la page d'adresse retombe sur le
	// même jeu de données, pas sur un millésime arbitraire.
	m.curAnnee = 0
	m.agrege = true
	m.chargerTout()
	m.data = m.total
	m.onglets = append([]string{"Adresse"}, ongletsStandard...)
	m.curOnglet = 0
	m.tri = TriDate
	m.curVente = m.premiereVenteAuNumero()
	m.ecran = ecranStats
	if m.pretVP {
		m.majStats()
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Entrées
// ---------------------------------------------------------------------------

func (m model) souris(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.ecran == ecranStats || m.ecran == ecranVente {
		return m.sourisTableau(msg)
	}
	if m.ecran != ecranCarte || m.carte.carte == nil {
		return m, nil
	}

	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		return m.deplacer(-1), nil
	case msg.Button == tea.MouseButtonWheelDown:
		return m.deplacer(1), nil
	case msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonNone:
		// Survol : la zone sous le pointeur s'éclaire, comme sur une vraie carte.
		if z := m.carte.carte.ZoneEn(msg.X-margeGauche, msg.Y-entetes); z >= 0 && z != m.carte.sel {
			m.carte.sel = z
			m.carte.suivre(m.carte.carte.H)
		}
		return m, nil

	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		z := m.carte.carte.ZoneEn(msg.X-margeGauche, msg.Y-entetes)
		if z < 0 {
			return m, nil
		}
		m.carte.sel = z
		m.carte.suivre(m.carte.carte.H)
		return m.ouvrir()
	}
	return m, nil
}

func (m model) touche(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// L'écran de recherche capte tout : sans cela, taper « q » dans un nom de
	// rue fermerait le programme.
	if m.ecran == ecranRecherche {
		return m.toucheRecherche(msg)
	}
	// Les écrans d'annonces ont leur propre clavier : le formulaire consomme
	// les chiffres, l'écran de zone consomme les lettres, et le tableau a ses
	// touches de tri.
	if m.ecran == ecranCarte && m.carte.saisitFiltre {
		return m.toucheFiltre(msg)
	}
	switch m.ecran {
	case ecranAnnonces:
		return m.toucheAnnonces(msg)
	case ecranZones:
		return m.toucheZones(msg)
	case ecranResultats:
		return m.toucheResultats(msg)
	}

	switch msg.String() {

	case "ctrl+c", "q":
		if m.tele.annuler != nil {
			m.tele.annuler()
		}
		return m, tea.Quit

	case "esc", "left", "h":
		// Un filtre posé se retire avant de remonter d'un échelon.
		if m.ecran == ecranCarte && m.carte.filtre != "" {
			return m.effacerFiltre()
		}
		return m.retour()

	case "up", "k":
		return m.monter(1)

	case "down", "j":
		return m.descendre(1)

	case "pgup":
		if m.ecran == ecranStats || m.ecran == ecranVente {
			m.vp.ViewUp()
			return m, nil
		}
		return m.monter(10)

	case "pgdown":
		if m.ecran == ecranStats || m.ecran == ecranVente {
			m.vp.ViewDown()
			return m, nil
		}
		return m.descendre(10)

	case " ":
		if m.ecran == ecranStats || m.ecran == ecranVente {
			m.vp.ViewDown()
		}
		return m, nil

	case "v", "V", "c":
		// Voir le bien : une vente est désignée aussi bien depuis la fiche que
		// depuis la ligne sélectionnée du tableau.
		if m.ecran != ecranVente && m.ecran != ecranStats {
			return m, nil
		}
		if vue, ok := VuePourTouche(msg.String()); ok {
			return m.ouvrirVue(vue)
		}
		return m, nil

	case "n":
		// Vente suivante sans repasser par le tableau.
		if m.ecran == ecranVente {
			return m.deplacerVente(1)
		}
		return m, nil

	case "p":
		if m.ecran == ecranVente {
			return m.deplacerVente(-1)
		}
		return m, nil

	case "s":
		if m.ecran == ecranStats && len(m.ventesOnglet()) > 0 {
			m.tri = (m.tri + 1) % Tri(len(LibellesTri))
			m.curVente = 0
			m.majStats()
			m.vp.GotoTop()
		}
		return m, nil

	case "enter", "right":
		return m.ouvrir()

	case "o":
		// Comme sur l'écran des annonces : ouvrir l'annonce sélectionnée.
		if m.ecran == ecranStats && m.onglets[m.curOnglet] == ongletMarche {
			return m.ouvrirAnnonceMarche()
		}
		return m, nil

	case "/":
		// Au niveau des communes, la liste est trop longue pour s'y déplacer
		// ligne à ligne : « / » la filtre. La recherche d'adresse y reste
		// accessible par « A ».
		if m.ecran == ecranCarte && m.carte.niveau == nivCommunes {
			return m.ouvrirFiltre()
		}
		if m.ecran == ecranCarte {
			return m.ouvrirRecherche()
		}
		return m, nil

	case "A":
		if m.ecran == ecranCarte {
			return m.ouvrirRecherche()
		}
		return m, nil

	case "a":
		// Les biens à vendre, par opposition aux ventes passées de DVF.
		if m.ecran == ecranCarte {
			return m.ouvrirAnnonces()
		}
		return m, nil

	case "l":
		if m.ecran == ecranCarte && len(m.villes) > 0 {
			m.ecran = ecranVilles
			m.sourceLocale = true
		}
		return m, nil

	case "r":
		// Forcer le rafraîchissement : les millésimes passés sont complétés à
		// chaque publication, un cache jugé frais peut être incomplet.
		if m.ecran == ecranCarte && m.carte.niveau == nivDepartements {
			if z, ok := m.carte.selection(); ok {
				m.oublierAdresse()
				return m.telecharger(z, true)
			}
		}
		return m, nil

	case "tab":
		if m.ecran == ecranStats {
			m.changerOnglet(1)
			return m, m.chargerMarche()
		}
		return m, nil

	case "shift+tab":
		if m.ecran == ecranStats {
			m.changerOnglet(-1)
			return m, m.chargerMarche()
		}
		return m, nil
	}

	return m, nil
}

// deplacer avance dans la liste affichée, qui peut être filtrée : on se
// déplace de rang en rang, pas d'indice en indice.
func (m model) deplacer(pas int) model {
	vis := m.carte.visibles()
	if len(vis) == 0 {
		return m
	}
	p := position(vis, m.carte.sel)
	if p < 0 {
		p = 0 // sélection hors liste : on repart de la première
	} else {
		p = borner(p+pas, len(vis))
	}
	m.carte.sel = vis[p]
	if m.carte.carte != nil {
		m.carte.suivre(m.carte.carte.H)
	}
	return m
}

func (m model) monter(pas int) (tea.Model, tea.Cmd) {
	switch m.ecran {
	case ecranCarte:
		return m.deplacer(-pas), nil
	case ecranVilles:
		m.curVille = borner(m.curVille-pas, len(m.villes))
	case ecranAnnees:
		m.curAnnee = borner(m.curAnnee-pas, len(m.listeAnnees()))
	case ecranStats:
		if m.onglets[m.curOnglet] == ongletMarche {
			return m.deplacerMarche(-pas)
		}
		if len(m.ventesOnglet()) > 0 {
			return m.deplacerVente(-pas)
		}
		m.vp.LineUp(pas)
	case ecranVente:
		m.vp.LineUp(pas)
	}
	return m, nil
}

func (m model) descendre(pas int) (tea.Model, tea.Cmd) {
	switch m.ecran {
	case ecranCarte:
		return m.deplacer(pas), nil
	case ecranVilles:
		m.curVille = borner(m.curVille+pas, len(m.villes))
	case ecranAnnees:
		m.curAnnee = borner(m.curAnnee+pas, len(m.listeAnnees()))
	case ecranStats:
		if m.onglets[m.curOnglet] == ongletMarche {
			return m.deplacerMarche(pas)
		}
		if len(m.ventesOnglet()) > 0 {
			return m.deplacerVente(pas)
		}
		m.vp.LineDown(pas)
	case ecranVente:
		m.vp.LineDown(pas)
	}
	return m, nil
}

func borner(v, n int) int {
	if v < 0 || n == 0 {
		return 0
	}
	if v >= n {
		return n - 1
	}
	return v
}

// ouvrir descend d'un niveau : c'est ici que se décide tout l'enchaînement.
func (m model) ouvrir() (tea.Model, tea.Cmd) {
	switch m.ecran {

	case ecranCarte:
		z, ok := m.carte.selection()
		if !ok {
			return m, nil
		}
		switch m.carte.niveau {
		case nivRegions:
			m.region = z
			m.ecran = ecranAttente
			m.attente = z.Nom + "…"
			return m, tea.Batch(m.spinner.Tick, cmdDepartements(z.Code))
		case nivDepartements:
			m.oublierAdresse()
			return m.telecharger(z, false)
		default:
			m.oublierAdresse()
			m.carte.saisitFiltre = false // un clic sur la carte peut ouvrir en pleine saisie
			m.ecran = ecranAttente
			m.attente = "Extraction de " + z.Nom + "…"
			return m, tea.Batch(m.spinner.Tick, cmdExtraire(m.racine, m.dep.Code, z))
		}

	case ecranVilles:
		if len(m.villes) == 0 {
			return m, nil
		}
		m.oublierAdresse()
		m.sourceLocale = true
		m.curAnnee = 0
		m.ecran = ecranAnnees
		return m, nil

	case ecranAnnees:
		m.chargerSelection()
		m.ecran = ecranStats
		m.curOnglet = 0
		m.curVente = 0
		m.tri = TriDate
		if m.pretVP {
			m.majStats()
			m.vp.GotoTop()
		}
		return m, nil

	case ecranStats:
		if m.onglets[m.curOnglet] == ongletMarche {
			return m.ouvrirAnnonceMarche()
		}
		if len(m.ventesOnglet()) == 0 {
			return m, nil
		}
		m.ecran = ecranVente
		if m.pretVP {
			m.vp.SetContent(m.contenuVente())
			m.vp.GotoTop()
		}
		return m, nil
	}
	return m, nil
}

// telecharger lance un lot de commandes, une par millésime. Bubble Tea les
// exécute en arrière-plan : l'interface reste vivante et annulable.
func (m model) telecharger(dep Zone, forcer bool) (tea.Model, tea.Cmd) {
	ctx, annuler := context.WithCancel(context.Background())

	m.dep = dep
	m.tele = etatTelechargement{
		dep:      dep,
		annees:   AnneesCandidates(),
		resultat: map[string]ResultatTelechargement{},
		annuler:  annuler,
	}
	m.ecran = ecranTelechargement
	m.alerte = ""

	cmds := []tea.Cmd{m.spinner.Tick}
	for _, a := range m.tele.annees {
		cmds = append(cmds, cmdTelecharger(ctx, a, dep.Code, forcer))
	}
	return m, tea.Batch(cmds...)
}

// retour remonte d'un écran.
func (m model) retour() (tea.Model, tea.Cmd) {
	switch m.ecran {

	case ecranVente:
		m.ecran = ecranStats
		if m.pretVP {
			m.majStats()
		}

	case ecranStats:
		m.ecran = ecranAnnees

	case ecranAnnees:
		if m.sourceLocale {
			m.ecran = ecranVilles
			break
		}
		m.ecran = ecranCarte
		// Une recherche d'adresse a pu changer de département sans passer par
		// la carte : celle qui est affichée décrirait alors un autre
		// département que celui dont on vient. On la redemande.
		if m.carte.niveau == nivCommunes && m.carte.dep != m.dep.Code {
			m.ecran = ecranAttente
			m.attente = "Contours des communes du " + m.dep.Code + "…"
			return m, tea.Batch(m.spinner.Tick, cmdCommunes(m.dep.Code))
		}

	case ecranRecherche:
		m.ecran = ecranCarte

	case ecranVilles:
		m.sourceLocale = false
		m.ecran = ecranCarte

	case ecranTelechargement:
		if m.tele.annuler != nil {
			m.tele.annuler()
		}
		m.ecran = ecranCarte

	case ecranCarte:
		switch m.carte.niveau {
		case nivDepartements:
			m.ecran = ecranAttente
			m.attente = "Chargement des contours…"
			return m, tea.Batch(m.spinner.Tick, cmdRegions())
		case nivCommunes:
			m.ecran = ecranAttente
			m.attente = m.region.Nom + "…"
			return m, tea.Batch(m.spinner.Tick, cmdDepartements(m.region.Code))
		}
	}
	return m, nil
}

// millesimes renvoie les millésimes de la source courante, du plus récent au
// plus ancien.
func (m model) millesimes() []string {
	if m.sourceLocale {
		if m.curVille >= len(m.villes) {
			return nil
		}
		var out []string
		for _, a := range m.villes[m.curVille].Annees {
			out = append(out, a.Nom)
		}
		return out
	}
	return m.annees
}

// entreeAgregat : vrai quand la liste s'ouvre sur une entrée « toutes les
// années ». Elle n'a d'intérêt qu'à partir de deux millésimes.
func (m model) entreeAgregat() bool { return len(m.millesimes()) > 1 }

// oublierAdresse remet le modèle dans son état ordinaire. Une page d'adresse
// détourne deux choses — l'enchaînement après téléchargement, et la liste des
// onglets — et les deux doivent disparaître dès qu'on repart par la carte, sans
// quoi le prochain département ouvert sauterait à la commune précédente.
func (m *model) oublierAdresse() {
	m.adresse = nil
	m.onglets = ongletsStandard
	m.curOnglet = borner(m.curOnglet, len(m.onglets))
}

// listeAnnees renvoie les entrées de l'écran de choix : l'agrégat d'abord, les
// millésimes ensuite. La période est un choix comme un autre — la mettre ici
// plutôt que dans un onglet garde les onglets sur le seul axe du type de bien.
func (m model) listeAnnees() []string {
	annees := m.millesimes()
	if !m.entreeAgregat() {
		return annees
	}
	label := fmt.Sprintf("Toutes les années  (%s → %s, %d millésimes)",
		annees[len(annees)-1], annees[0], len(annees))
	return append([]string{label}, annees...)
}

// agregatChoisi : le curseur est sur l'entrée « toutes les années ».
func (m model) agregatChoisi() bool { return m.entreeAgregat() && m.curAnnee == 0 }

// indexMillesime convertit la position du curseur en indice de millésime.
func (m model) indexMillesime() int {
	if m.entreeAgregat() {
		return m.curAnnee - 1
	}
	return m.curAnnee
}

// chargerTout lit tous les millésimes disponibles de la source courante et les
// fusionne. C'est ce qui alimente l'entrée « toutes les années » — et la page
// d'une adresse, qui n'aurait aucun sens sur un seul millésime : un bien se
// vend une fois par décennie.
func (m *model) chargerTout() {
	var jeux []Dataset
	if m.sourceLocale {
		if m.curVille >= len(m.villes) {
			return
		}
		v := m.villes[m.curVille]
		for _, a := range v.Annees {
			if d, err := ChargerDataset(v.Nom, a.Nom, a.Chemin); err == nil {
				jeux = append(jeux, d)
			}
		}
	} else {
		for _, a := range m.annees {
			if d, err := ChargerDataset(m.commune.Nom, a, m.dossierCommune(a)); err == nil {
				jeux = append(jeux, d)
			}
		}
	}
	sort.Slice(jeux, func(i, j int) bool { return jeux[i].Annee < jeux[j].Annee })
	m.parAnnee = jeux
	m.total = Fusionner(jeux)
}

// chargerSelection charge ce que désigne le curseur de la liste : un millésime,
// ou leur fusion. Les onglets qui suivent n'ont pas à savoir lequel des deux.
func (m *model) chargerSelection() {
	m.agrege = m.agregatChoisi()
	if m.agrege {
		m.chargerTout()
		m.data, m.erreur = m.total, nil
		return
	}
	// L'agrégat n'est plus affiché : on l'oublie, sauf si une page d'adresse
	// est ouverte — elle décrit toujours toute la période, quel que soit le
	// millésime choisi pour les autres onglets.
	if m.adresse == nil {
		m.parAnnee, m.total = nil, Dataset{}
	} else if len(m.total.Sales) == 0 {
		m.chargerTout()
	}
	m.chargerAnnee()
}

func (m *model) chargerAnnee() {
	i := m.indexMillesime()
	if i < 0 {
		return
	}
	if m.sourceLocale {
		if m.curVille >= len(m.villes) {
			return
		}
		v := m.villes[m.curVille]
		if i >= len(v.Annees) {
			return
		}
		a := v.Annees[i]
		m.data, m.erreur = ChargerDataset(v.Nom, a.Nom, a.Chemin)
		return
	}

	if i >= len(m.annees) {
		return
	}
	annee := m.annees[i]
	m.data, m.erreur = ChargerDataset(m.commune.Nom, annee, m.dossierCommune(annee))
}

// ---------------------------------------------------------------------------
// Rendu
// ---------------------------------------------------------------------------

func (m model) View() string {
	switch m.ecran {

	case ecranAttente:
		return fmt.Sprintf("\n  %s %s\n\n  %s\n",
			m.spinner.View(), m.attente,
			styleAide.Render("esc retour · q quitter"))

	case ecranTelechargement:
		return m.tele.vue(m.spinner)

	case ecranCarte:
		vue := m.vueCarte()
		if m.alerte != "" {
			vue += "\n  " + styleWarn.Render(m.alerte)
		}
		return vue

	case ecranVilles:
		return m.vueListe("collection locale", listeNoms(m.villes), m.curVille,
			"↑/↓ naviguer · ↵ ouvrir · esc carte · q quitter")

	case ecranAnnees:
		titre := m.commune.Nom
		if m.sourceLocale && m.curVille < len(m.villes) {
			titre = m.villes[m.curVille].Nom
		}
		return m.vueListe(titre+" — millésimes", m.listeAnnees(), m.curAnnee,
			"↑/↓ naviguer · ↵ ouvrir · esc retour · q quitter")

	case ecranVente:
		return m.vueVente()

	case ecranRecherche:
		return m.vueRecherche()

	case ecranAnnonces:
		return m.vueAnnonces()

	case ecranZones:
		return m.vueZones()

	case ecranResultats:
		return m.vueResultats()

	default:
		return m.vueStats()
	}
}

func listeNoms(villes []Ville) []string {
	var noms []string
	for _, v := range villes {
		suffixe := fmt.Sprintf("  (%d année", len(v.Annees))
		if len(v.Annees) > 1 {
			suffixe += "s"
		}
		suffixe += ")"
		noms = append(noms, v.Nom+suffixe)
	}
	return noms
}

func (m model) vueListe(titre string, items []string, curseur int, aide string) string {
	var b strings.Builder
	b.WriteString("\n  " + styleTitre.Render("DVF — "+titre) + "\n\n")

	if len(items) == 0 {
		b.WriteString("  " + styleWarn.Render("Aucun dossier trouvé.") + "\n")
	}
	for i, it := range items {
		if i == curseur {
			b.WriteString("  " + styleSel.Render(" ▸ "+it+" ") + "\n")
		} else {
			b.WriteString("    " + it + "\n")
		}
	}
	b.WriteString("\n  " + styleAide.Render(aide) + "\n")
	return b.String()
}

func (m model) vueStats() string {
	var b strings.Builder

	// Le titre suit l'onglet : un millésime pour les onglets qui en décrivent
	// un, la période complète pour ceux qui agrègent.
	titre := fmt.Sprintf("%s — %s", m.data.Ville, m.jeu().Annee)
	if m.agrege && len(m.parAnnee) > 1 {
		titre += fmt.Sprintf(" (%d millésimes)", len(m.parAnnee))
	}
	if m.onglets[m.curOnglet] == "Adresse" && m.adresse != nil {
		titre = fmt.Sprintf("%s — %s", m.adresse.Label, m.jeu().Annee)
	}
	ligne := "  " + styleTitre.Render(titre)
	// Les dates partagent la ligne du titre : une ligne de plus décalerait la
	// zone de défilement. Faute de place, elles cèdent.
	if dates := datesDonnees(m.jeu()); dates != "" {
		avec := ligne + "   " + styleDim.Render(dates)
		if m.largeur == 0 || lipgloss.Width(avec) <= m.largeur {
			ligne = avec
		}
	}
	b.WriteString("\n" + ligne + "\n")

	var tabs []string
	for i, o := range m.onglets {
		if i == m.curOnglet {
			tabs = append(tabs, styleTabOn.Render(o))
		} else {
			tabs = append(tabs, styleTab.Render(o))
		}
	}
	b.WriteString("  " + lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...) + "\n")

	if m.pretVP {
		b.WriteString(m.vp.View())
	} else {
		b.WriteString(m.contenuStats())
	}

	aide := "tab onglet · ↑/↓ défiler · esc retour · q quitter"
	if lignes, _, _ := m.lignesMarche(); m.onglets[m.curOnglet] == ongletMarche && len(lignes) > 0 {
		aide = "↑/↓ annonce · ↵ ou o ouvrir l'annonce dans le navigateur · tab onglet · esc retour"
	}
	if len(m.ventesOnglet()) > 0 {
		aide = fmt.Sprintf("↑/↓ vente · ↵ détail · v carte · c cadastre · V rue · s tri (%s) · tab onglet · esc retour",
			LibellesTri[m.tri])
	}
	if m.noteVue != "" {
		b.WriteString("\n  " + styleDim.Render(m.noteVue))
	}
	b.WriteString("\n  " + styleAide.Render(aide))
	return b.String()
}

// datesDonnees résume la fraîcheur d'un jeu : jusqu'où vont les ventes, et de
// quand date la publication dont elles proviennent.
func datesDonnees(d Dataset) string {
	var parts []string
	if !d.DerniereVente.IsZero() {
		parts = append(parts, "ventes jusqu'au "+d.DerniereVente.Format("02/01/2006"))
	}
	if !d.Publication.IsZero() {
		parts = append(parts, "publié le "+d.Publication.Local().Format("02/01/2006"))
	}
	if d.Provisoire {
		parts = append(parts, "fichier brut DGFiP, non géolocalisé")
	}
	return strings.Join(parts, " · ")
}

func (m *model) contenuStats() string {
	if m.erreur != nil {
		return "\n  " + styleWarn.Render("Erreur : "+m.erreur.Error()) + "\n"
	}
	switch m.onglets[m.curOnglet] {
	case "Maisons":
		return m.vueTypeBien("Maison")
	case "Appartements":
		return m.vueTypeBien("Appartement")
	case "Terrains":
		return m.vueTerrains()
	case "Adresse":
		return m.vueAdresse()
	case ongletMarche:
		return m.vueMarche()
	default:
		return m.vueResume()
	}
}

func (m model) vueResume() string {
	d := m.data
	var b strings.Builder

	b.WriteString("\n")
	if m.agrege {
		b.WriteString("  " + styleSection.Render(fmt.Sprintf(
			"%d MILLÉSIMES FUSIONNÉS — %s", len(m.parAnnee), d.Annee)) + "\n\n")
	}
	b.WriteString(fmt.Sprintf("  %s  ·  %s  ·  %s\n",
		styleDim.Render(pluriel(d.Fichiers, "fichier")),
		styleDim.Render(pluriel(d.Mutations, "mutation")),
		styleOK.Render(pluriel(len(d.Sales), "vente")+" exploitable"+marqueS(len(d.Sales)))))
	if d.Ecartees > 0 {
		b.WriteString("  " + styleWarn.Render(fmt.Sprintf(
			"%s écartée%s (ventes globales, échanges, prix absent)",
			pluriel(d.Ecartees, "mutation"), marqueS(d.Ecartees))) + "\n")
	}

	b.WriteString("\n  " + styleSection.Render("MARCHÉ PAR TYPE DE BIEN") + "\n\n")
	b.WriteString(enteteTypes + "\n")
	for _, r := range ResumerParType(d.Sales) {
		b.WriteString(ligneType(r) + "\n")
	}

	// Sur un jeu fusionné, l'évolution millésime par millésime vient ici : elle
	// dit ce que la fusion elle-même dissimule.
	m.ecrireEvolution(&b)

	// Activité mensuelle : révèle la saisonnalité et la tension du marché.
	b.WriteString("\n  " + styleSection.Render("ACTIVITÉ PAR MOIS") + "\n\n")
	mois := VentesParMois(d.Sales)
	maxM := 0
	for _, n := range mois {
		if n > maxM {
			maxM = n
		}
	}
	noms := []string{"Jan", "Fév", "Mar", "Avr", "Mai", "Juin", "Juil", "Aoû", "Sep", "Oct", "Nov", "Déc"}
	for i, n := range mois {
		barre := ""
		if maxM > 0 {
			barre = strings.Repeat("█", n*30/maxM)
		}
		b.WriteString(fmt.Sprintf("  %-5s %-32s %3d\n", noms[i], barre, n))
	}

	// Voies les plus actives : repérage des micro-marchés.
	voies := TopVoies(d.Sales, 8)
	if len(voies) > 0 {
		b.WriteString("\n  " + styleSection.Render("VOIES LES PLUS ACTIVES") + "\n\n")
		b.WriteString(styleDim.Render("  Voie                                Ventes    Prix médian") + "\n")
		for _, v := range voies {
			nom := v.Nom
			if len(nom) > 34 {
				nom = nom[:34]
			}
			b.WriteString(fmt.Sprintf("  %-34s %6d  %13s\n", nom, v.Nb, euros(v.PrixMed)))
		}
	}

	b.WriteString("\n  " + styleDim.Render(
		"Rappel : les valeurs foncières DVF n'incluent pas les frais de notaire.") + "\n")
	return b.String()
}

func (m *model) vueTypeBien(typeBien string) string {
	st := StatsPourType(m.data.Sales, typeBien)
	var b strings.Builder

	b.WriteString("\n")
	if st.Nb == 0 {
		b.WriteString("  " + styleWarn.Render("Aucune vente de ce type sur la période.") + "\n")
		return b.String()
	}

	b.WriteString("  " + styleSection.Render(fmt.Sprintf("%s — %d ventes", strings.ToUpper(typeBien), st.Nb)) + "\n\n")

	// La médiane est plus fiable que la moyenne : elle résiste aux ventes
	// atypiques (biens d'exception, ventes entre proches).
	b.WriteString(fmt.Sprintf("  %-22s %s\n", "Prix médian", styleOK.Render(euros(st.Prix.Med))))
	b.WriteString(fmt.Sprintf("  %-22s %s\n", "Prix moyen", euros(st.Prix.Moy)))
	b.WriteString(fmt.Sprintf("  %-22s %s → %s   %s\n", "Cœur du marché",
		euros(st.Prix.Q1), euros(st.Prix.Q3), styleDim.Render("(50 % des ventes)")))
	b.WriteString(fmt.Sprintf("  %-22s %s → %s\n", "Extrêmes", euros(st.Prix.Min), euros(st.Prix.Max)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %-22s %s   %s\n", "Prix au m² médian",
		styleOK.Render(eurosM2(st.PrixM2.Med)),
		styleDim.Render(fmt.Sprintf("(%s → %s)", eurosM2(st.PrixM2.Q1), eurosM2(st.PrixM2.Q3)))))
	b.WriteString(fmt.Sprintf("  %-22s %s   %s\n", "Surface bâtie médiane", m2(st.Surface.Med),
		styleDim.Render(fmt.Sprintf("(%s → %s)", m2(st.Surface.Q1), m2(st.Surface.Q3)))))
	if st.Terrain.Nb > 0 {
		b.WriteString(fmt.Sprintf("  %-22s %s   %s\n", "Terrain médian", m2(st.Terrain.Med),
			styleDim.Render(fmt.Sprintf("(%s → %s)", m2(st.Terrain.Q1), m2(st.Terrain.Q3)))))
	}

	if tranches := ParTrancheSurface(m.data.Sales, typeBien); len(tranches) > 0 {
		b.WriteString("\n  " + styleSection.Render("PAR TRANCHE DE SURFACE") + "\n\n")
		b.WriteString(styleDim.Render("  Tranche          Ventes    Prix médian     Médiane €/m²") + "\n")
		for _, t := range tranches {
			b.WriteString(fmt.Sprintf("  %-15s %6d  %13s  %15s\n",
				t.Label, t.Nb, euros(t.PrixMed), eurosM2(t.PrixM2Med)))
		}
	}

	if pieces := ParPieces(m.data.Sales, typeBien); len(pieces) > 0 {
		b.WriteString("\n  " + styleSection.Render("PAR NOMBRE DE PIÈCES") + "\n\n")
		b.WriteString(styleDim.Render("  Pièces           Ventes    Prix médian     Médiane €/m²") + "\n")
		for _, t := range pieces {
			b.WriteString(fmt.Sprintf("  %-15s %6d  %13s  %15s\n",
				t.Label, t.Nb, euros(t.PrixMed), eurosM2(t.PrixM2Med)))
		}
	}

	m.ecrireTableau(&b, m.ventesOnglet())
	return b.String()
}

func (m *model) vueTerrains() string {
	prix, pm2 := StatsTerrain(m.data.Sales)
	var b strings.Builder

	b.WriteString("\n")
	if prix.Nb == 0 {
		b.WriteString("  " + styleWarn.Render("Aucune vente de terrain nu sur la période.") + "\n")
		return b.String()
	}

	b.WriteString("  " + styleSection.Render(fmt.Sprintf("TERRAINS NUS — %d ventes", prix.Nb)) + "\n\n")
	b.WriteString(fmt.Sprintf("  %-22s %s\n", "Prix médian", styleOK.Render(euros(prix.Med))))
	b.WriteString(fmt.Sprintf("  %-22s %s → %s   %s\n", "Cœur du marché",
		euros(prix.Q1), euros(prix.Q3), styleDim.Render("(50 % des ventes)")))
	b.WriteString(fmt.Sprintf("  %-22s %s → %s\n", "Extrêmes", euros(prix.Min), euros(prix.Max)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %-22s %s   %s\n", "Prix au m² médian",
		styleOK.Render(eurosM2(pm2.Med)),
		styleDim.Render(fmt.Sprintf("(%s → %s)", eurosM2(pm2.Q1), eurosM2(pm2.Q3)))))

	b.WriteString("\n  " + styleDim.Render(
		"Le prix au m² de terrain nu sert de repère pour un projet de construction.") + "\n")
	b.WriteString("  " + styleDim.Render(
		"Attention : il mélange terrains constructibles et parcelles agricoles ou boisées,") + "\n")
	b.WriteString("  " + styleDim.Render(
		"dont les prix sont sans commune mesure. La dispersion Q1–Q3 le reflète.") + "\n")

	m.ecrireTableau(&b, m.ventesOnglet())
	return b.String()
}

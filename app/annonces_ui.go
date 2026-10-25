package main

// ---------------------------------------------------------------------------
// Les trois écrans des annonces
// ---------------------------------------------------------------------------
//
//	critères  →  choix de zone  →  résultats
//
// Le formulaire est un écran à part entière et non une barre de filtres posée
// au-dessus du tableau : une recherche d'annonces a une dizaine de paramètres,
// et une barre les rendrait tous illisibles pour n'en montrer aucun. On paye ce
// choix d'un aller-retour quand on ajuste un critère — d'où la touche « f »
// depuis les résultats, qui ramène au formulaire sans le vider.

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Le formulaire
// ---------------------------------------------------------------------------

// natureChamp dit comment une ligne du formulaire réagit aux touches. Décrire
// le formulaire par une table plutôt que par une suite de `if` : ajouter un
// critère devient une ligne, et la navigation, la saisie et le rendu n'ont pas
// à être retouchés.
type natureChamp int

const (
	champZone    natureChamp = iota // ↵ ouvre l'écran de choix de zone
	champNombre                     // saisie chiffrée
	champBascule                    // espace coche / décoche
	champAction                     // ↵ lance la recherche
)

type champAnnonce struct {
	Libelle string
	Nature  natureChamp
	Unite   string
	Aide    string
}

// champsAnnonces décrit le formulaire dans l'ordre d'affichage. L'ordre suit
// celui dans lequel on cherche vraiment un bien : d'abord où, puis quoi, puis
// combien — pas l'ordre des champs de la struct Criteres.
var champsAnnonces = []champAnnonce{
	{Libelle: "Zone", Nature: champZone, Aide: "région, département ou commune"},
	{Libelle: "Rayon max", Nature: champNombre, Unite: "km", Aide: "autour du centre de la première zone"},
	{Libelle: "Maisons", Nature: champBascule},
	{Libelle: "Appartements", Nature: champBascule},
	{Libelle: "Terrains nus", Nature: champBascule},
	{Libelle: "Prix min", Nature: champNombre, Unite: "€"},
	{Libelle: "Prix max", Nature: champNombre, Unite: "€"},
	{Libelle: "Surface min", Nature: champNombre, Unite: "m²"},
	{Libelle: "Surface max", Nature: champNombre, Unite: "m²"},
	{Libelle: "Terrain min", Nature: champNombre, Unite: "m²"},
	{Libelle: "Terrain max", Nature: champNombre, Unite: "m²"},
	{Libelle: "Pièces min", Nature: champNombre},
	{Libelle: "Exclure le neuf", Nature: champBascule},
	{Libelle: "Nombre max de résultats", Nature: champNombre},
	{Libelle: "Lancer la recherche", Nature: champAction},
}

// Index des champs, pour ne pas écrire de nombres nus dans le code.
const (
	iZone = iota
	iRayon
	iMaisons
	iApparts
	iTerrains
	iPrixMin
	iPrixMax
	iSurfMin
	iSurfMax
	iTerrMin
	iTerrMax
	iPieces
	iSansNeuf
	iLimite
)

// formAnnonces est l'état de saisie.
//
// Les valeurs sont conservées en texte, pas en nombres. C'est délibéré : un
// champ où l'on vient d'effacer le dernier chiffre ne vaut pas zéro, il est
// vide — et un formulaire qui réaffiche « 0 » dès qu'on efface est
// insupportable à corriger. La conversion se fait une seule fois, au moment de
// construire les critères.
type formAnnonces struct {
	curseur  int
	valeurs  map[int]string
	bascules map[int]bool
	zones    []ZoneAnnonce
}

func nouveauFormAnnonces() formAnnonces {
	return formAnnonces{
		valeurs: map[int]string{},
		bascules: map[int]bool{
			iMaisons: true, iApparts: true, iTerrains: true,
		},
	}
}

// Criteres traduit le formulaire. Toute la conversion texte → nombre est ici,
// et elle ne peut pas échouer : un champ illisible vaut « pas de contrainte ».
// Refuser de lancer la recherche parce qu'un champ contient une lettre serait
// une rigueur mal placée dans un formulaire d'exploration.
func (f formAnnonces) Criteres() Criteres {
	c := Criteres{
		Zones:      f.zones,
		RayonKm:    entierOuZero(f.valeurs[iRayon]),
		PrixMin:    entierOuZero(f.valeurs[iPrixMin]),
		PrixMax:    entierOuZero(f.valeurs[iPrixMax]),
		SurfaceMin: entierOuZero(f.valeurs[iSurfMin]),
		SurfaceMax: entierOuZero(f.valeurs[iSurfMax]),
		TerrainMin: entierOuZero(f.valeurs[iTerrMin]),
		TerrainMax: entierOuZero(f.valeurs[iTerrMax]),
		PiecesMin:  int(entierOuZero(f.valeurs[iPieces])),
		Limite:     int(entierOuZero(f.valeurs[iLimite])),

		ExclureNeuf: f.bascules[iSansNeuf],
	}

	if f.bascules[iMaisons] {
		c.Types = append(c.Types, BienMaison)
	}
	if f.bascules[iApparts] {
		c.Types = append(c.Types, BienAppartement)
	}
	if f.bascules[iTerrains] {
		c.Types = append(c.Types, BienTerrain)
	}

	// Le rayon a besoin d'un centre. La première zone choisie le fournit : on
	// ne demande pas à l'utilisateur des coordonnées qu'il n'a pas, et cette
	// règle simple se dit en une ligne d'aide à l'écran.
	if c.RayonKm > 0 && len(f.zones) > 0 {
		c.CentreLat, c.CentreLon = f.zones[0].Lat, f.zones[0].Lon
		if c.CentreLat == 0 && c.CentreLon == 0 {
			c.RayonKm = 0 // zone sans position : mieux vaut ignorer le rayon que mentir
		}
	}
	return c
}

// valeurAffichee rend la valeur d'un champ telle qu'elle apparaît.
func (f formAnnonces) valeurAffichee(i int) string {
	switch champsAnnonces[i].Nature {

	case champZone:
		if len(f.zones) == 0 {
			return "toute la France"
		}
		noms := make([]string, 0, len(f.zones))
		for _, z := range f.zones {
			noms = append(noms, z.String())
		}
		if len(noms) > 2 {
			return fmt.Sprintf("%s, %s +%d", noms[0], noms[1], len(noms)-2)
		}
		return strings.Join(noms, ", ")

	case champBascule:
		if f.bascules[i] {
			return "[×]"
		}
		return "[ ]"

	case champAction:
		return "↵"

	default:
		v := f.valeurs[i]
		if v == "" {
			return "—"
		}
		if u := champsAnnonces[i].Unite; u != "" {
			return v + " " + u
		}
		return v
	}
}

// nouvelleSaisieZone : un champ distinct de celui de la recherche d'adresse.
// Partager le même reviendrait à voir sa rue apparaître dans un champ de zone,
// et à perdre l'un en utilisant l'autre.
func nouvelleSaisieZone() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Landes, Bayonne, Nouvelle-Aquitaine…"
	ti.Prompt = "  ▸ "
	ti.CharLimit = 80
	return ti
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type msgZonesAnnonces struct {
	requete string
	zones   []ZoneAnnonce
	err     error
}

type msgAnnonces struct {
	res ResultatAnnonces
	err error
}

func cmdResoudreZones(ctx context.Context, requete string) tea.Cmd {
	return func() tea.Msg {
		zs, err := ResoudreZones(ctx, requete)
		return msgZonesAnnonces{requete: requete, zones: zs, err: err}
	}
}

func cmdChercherAnnonces(ctx context.Context, c Criteres) tea.Cmd {
	return func() tea.Msg {
		res, err := ChercherAnnonces(ctx, c)
		return msgAnnonces{res: res, err: err}
	}
}

// ---------------------------------------------------------------------------
// Entrée dans les écrans
// ---------------------------------------------------------------------------

// ouvrirAnnonces affiche le formulaire, prérempli avec la zone sous le curseur
// de la carte quand il y en a une.
//
// Préremplir plutôt que présenter un formulaire vide : quand on appuie sur « a »
// en survolant les Landes, on veut les annonces des Landes. Le nom de la zone
// suffit — sa traduction en identifiant de portail est asynchrone, et c'est
// justement ce que fait cmdResoudreZones.
func (m model) ouvrirAnnonces() (tea.Model, tea.Cmd) {
	if m.formAnn.valeurs == nil {
		m.formAnn = nouveauFormAnnonces()
	}
	m.ecran = ecranAnnonces
	m.alerte = ""

	if len(m.formAnn.zones) == 0 {
		if z, ok := m.carte.selection(); ok && z.Nom != "" {
			m.saisieZone.SetValue(z.Nom)
			return m.ouvrirChoixZone()
		}
	}
	return m, nil
}

// ouvrirChoixZone bascule sur l'écran de saisie de zone et lance la résolution
// si le champ est déjà rempli.
func (m model) ouvrirChoixZone() (tea.Model, tea.Cmd) {
	m.ecran = ecranZones
	m.saisieZone.Focus()
	m.saisieZone.CursorEnd()
	m.alerte = ""

	requete := strings.TrimSpace(m.saisieZone.Value())
	if requete == "" || requete == m.requeteZone {
		return m, textinput.Blink
	}
	m.chercheZone = true
	m.requeteZone = requete
	return m, tea.Batch(m.spinner.Tick, textinput.Blink,
		cmdResoudreZones(context.Background(), requete))
}

// lancerAnnonces part chercher. Le contexte est conservé dans le modèle : sans
// lui, échap laisserait tourner des requêtes dont plus personne ne veut, et la
// réponse arriverait sur un écran qui n'existe plus.
func (m model) lancerAnnonces() (tea.Model, tea.Cmd) {
	if m.annulerAnn != nil {
		m.annulerAnn()
	}
	ctx, annuler := context.WithCancel(context.Background())

	m.annulerAnn = annuler
	m.critAnn = m.formAnn.Criteres()
	m.critAnn.Tri = m.triAnn
	m.chercheAnn = true
	m.alerte = ""
	m.noteVue = ""
	m.ecran = ecranResultats
	m.curAnnonce = 0

	return m, tea.Batch(m.spinner.Tick, cmdChercherAnnonces(ctx, m.critAnn))
}

// ---------------------------------------------------------------------------
// Réception
// ---------------------------------------------------------------------------

func (m model) appliquerZonesAnnonces(msg msgZonesAnnonces) (tea.Model, tea.Cmd) {
	if m.ecran != ecranZones {
		return m, nil // réponse arrivée après qu'on a quitté l'écran
	}
	m.chercheZone = false
	if msg.err != nil {
		m.alerte = msg.err.Error()
		m.zonesTrouvees = nil
		return m, nil
	}
	m.zonesTrouvees = msg.zones
	m.curZoneTrouvee = 0
	if len(m.zonesTrouvees) == 0 {
		m.alerte = "Aucune zone trouvée pour « " + msg.requete + " »."
	} else {
		m.alerte = ""
	}
	return m, nil
}

func (m model) appliquerAnnonces(msg msgAnnonces) (tea.Model, tea.Cmd) {
	m.chercheAnn = false
	if msg.err != nil {
		m.alerte = msg.err.Error()
		m.annonces = nil
		m.totalAnn = 0
		return m, nil
	}
	m.annonces = msg.res.Annonces
	m.totalAnn = msg.res.Total
	m.erreursAnn = msg.res.Erreurs
	m.curAnnonce = 0
	if len(m.annonces) == 0 {
		m.alerte = "Aucun bien ne correspond. Élargis un critère."
	} else {
		m.alerte = ""
	}
	if m.pretVP {
		m.vp.SetContent(m.contenuAnnonces())
		m.vp.GotoTop()
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Touches
// ---------------------------------------------------------------------------

// toucheAnnonces : le formulaire. Les chiffres tapés vont dans le champ courant
// sans qu'aucun d'eux n'ait à être « focalisé » : un champ numérique n'a pas
// besoin d'un curseur clignotant, il lui suffit d'être la ligne sélectionnée.
func (m model) toucheAnnonces(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.formAnn
	champ := champsAnnonces[f.curseur]

	switch msg.String() {

	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc", "left":
		m.ecran = ecranCarte
		return m, nil

	case "up", "shift+tab":
		f.curseur = borner(f.curseur-1, len(champsAnnonces))
		return m, nil

	case "down", "tab":
		f.curseur = borner(f.curseur+1, len(champsAnnonces))
		return m, nil

	case " ":
		if champ.Nature == champBascule {
			f.bascules[f.curseur] = !f.bascules[f.curseur]
		}
		return m, nil

	case "enter", "right":
		switch champ.Nature {
		case champZone:
			return m.ouvrirChoixZone()
		case champBascule:
			f.bascules[f.curseur] = !f.bascules[f.curseur]
			return m, nil
		default:
			return m.lancerAnnonces()
		}

	case "backspace":
		if champ.Nature == champNombre {
			v := f.valeurs[f.curseur]
			if v != "" {
				f.valeurs[f.curseur] = v[:len(v)-1]
			}
		}
		if champ.Nature == champZone {
			f.zones = nil
		}
		return m, nil
	}

	// Les chiffres, et rien d'autre : accepter des lettres ici obligerait à
	// afficher un message d'erreur qu'un champ « prix max » ne devrait jamais
	// avoir à afficher.
	if champ.Nature == champNombre && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if r >= '0' && r <= '9' {
			f.valeurs[f.curseur] += string(r)
		}
	}
	return m, nil
}

// toucheZones : l'écran de choix de zone. Comme la recherche d'adresse, il
// capte toutes les touches — sinon on ne pourrait pas taper « q » dans
// « Quimper ».
func (m model) toucheZones(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.saisieZone.Blur()
		m.ecran = ecranAnnonces
		m.alerte = ""
		return m, nil

	case "up":
		m.curZoneTrouvee = borner(m.curZoneTrouvee-1, len(m.zonesTrouvees))
		return m, nil

	case "down":
		m.curZoneTrouvee = borner(m.curZoneTrouvee+1, len(m.zonesTrouvees))
		return m, nil

	case " ":
		// Cumuler les zones : « les Landes et les Pyrénées-Atlantiques » est
		// une recherche parfaitement légitime, et le portail sait unir
		// plusieurs zones en une requête.
		if m.curZoneTrouvee < len(m.zonesTrouvees) {
			m.formAnn.zones = basculerZone(m.formAnn.zones, m.zonesTrouvees[m.curZoneTrouvee])
		}
		return m, nil

	case "enter":
		requete := strings.TrimSpace(m.saisieZone.Value())
		if requete != "" && requete != m.requeteZone {
			m.chercheZone = true
			m.requeteZone = requete
			m.alerte = ""
			return m, tea.Batch(m.spinner.Tick,
				cmdResoudreZones(context.Background(), requete))
		}
		if m.curZoneTrouvee < len(m.zonesTrouvees) {
			m.formAnn.zones = basculerZone(m.formAnn.zones, m.zonesTrouvees[m.curZoneTrouvee])
		}
		m.saisieZone.Blur()
		m.ecran = ecranAnnonces
		return m, nil
	}

	var cmd tea.Cmd
	m.saisieZone, cmd = m.saisieZone.Update(msg)
	return m, cmd
}

// basculerZone ajoute ou retire une zone de la sélection.
func basculerZone(zones []ZoneAnnonce, z ZoneAnnonce) []ZoneAnnonce {
	for i, v := range zones {
		if v.Source == z.Source && v.ID == z.ID {
			return append(zones[:i], zones[i+1:]...)
		}
	}
	return append(zones, z)
}

func zoneChoisie(zones []ZoneAnnonce, z ZoneAnnonce) bool {
	for _, v := range zones {
		if v.Source == z.Source && v.ID == z.ID {
			return true
		}
	}
	return false
}

// toucheResultats : le tableau.
func (m model) toucheResultats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "ctrl+c", "q":
		if m.annulerAnn != nil {
			m.annulerAnn()
		}
		return m, tea.Quit

	case "esc", "left", "h", "f":
		if m.annulerAnn != nil {
			m.annulerAnn()
		}
		m.ecran = ecranAnnonces
		return m, nil

	case "up", "k":
		return m.deplacerAnnonce(-1)

	case "down", "j":
		return m.deplacerAnnonce(1)

	case "pgup":
		m.vp.ViewUp()
		return m, nil

	case "pgdown", " ":
		m.vp.ViewDown()
		return m, nil

	case "s":
		// Changer de critère de tri. Le tri par prix est celui qu'on veut
		// neuf fois sur dix : c'est donc lui qui est en place au départ, et
		// « s » sert à en sortir, pas à y arriver.
		m.triAnn = m.triAnn.Suivant()
		return m.retrier()

	case "i":
		m.triAnn = m.triAnn.Inverse()
		return m.retrier()

	case "enter", "o":
		if a, ok := m.annonceCourante(); ok {
			m.noteVue = "Ouverture de l'annonce…"
			return m, cmdOuvrirVue(a.URL)
		}
		return m, nil

	case "r":
		return m.lancerAnnonces()
	}
	return m, nil
}

// retrier réordonne sans relancer de requête.
//
// C'est le bénéfice de garder les annonces en mémoire plutôt que de redemander
// au portail : changer de tri est instantané. Mais le tri ne porte que sur ce
// qui a été rapporté — d'où l'avertissement affiché sous le tableau quand le
// total du portail dépasse ce qu'on tient.
func (m model) retrier() (tea.Model, tea.Cmd) {
	TrierAnnonces(m.annonces, m.triAnn)
	m.curAnnonce = 0
	if m.pretVP {
		m.vp.SetContent(m.contenuAnnonces())
		m.vp.GotoTop()
	}
	return m, nil
}

func (m model) deplacerAnnonce(pas int) (tea.Model, tea.Cmd) {
	m.curAnnonce = borner(m.curAnnonce+pas, len(m.annonces))
	if m.pretVP {
		m.vp.SetContent(m.contenuAnnonces())
	}
	return m, nil
}

func (m model) annonceCourante() (Annonce, bool) {
	if m.curAnnonce < 0 || m.curAnnonce >= len(m.annonces) {
		return Annonce{}, false
	}
	return m.annonces[m.curAnnonce], true
}

// ---------------------------------------------------------------------------
// Rendu
// ---------------------------------------------------------------------------

func (m model) vueAnnonces() string {
	var b strings.Builder
	b.WriteString("\n  " + styleTitre.Render("Biens en vente — critères") + "\n\n")

	for i, c := range champsAnnonces {
		valeur := m.formAnn.valeurAffichee(i)
		ligne := fmt.Sprintf("%-24s %s", c.Libelle, valeur)

		if i == m.formAnn.curseur {
			b.WriteString("  " + styleSel.Render(" ▸ "+ligne+" "))
			if c.Aide != "" {
				b.WriteString("  " + styleDim.Render(c.Aide))
			}
			b.WriteString("\n")
		} else {
			b.WriteString("     " + ligne + "\n")
		}
	}

	if m.alerte != "" {
		b.WriteString("\n  " + styleWarn.Render(m.alerte) + "\n")
	}

	b.WriteString("\n  " + styleAide.Render(
		"↑/↓ champ · chiffres pour saisir · espace cocher · ↵ ouvrir/lancer · esc carte") + "\n")
	b.WriteString("  " + styleDim.Render(
		"Un champ vide vaut « pas de contrainte ». Le rayon s'applique autour de la première zone.") + "\n")

	noms := make([]string, 0, len(Sources()))
	for _, s := range Sources() {
		noms = append(noms, s.Nom())
	}
	b.WriteString("  " + styleDim.Render("Sources interrogées : "+strings.Join(noms, ", ")) + "\n")
	return b.String()
}

func (m model) vueZones() string {
	var b strings.Builder
	b.WriteString("\n  " + styleTitre.Render("Biens en vente — où chercher") + "\n\n")
	b.WriteString(m.saisieZone.View() + "\n\n")

	switch {
	case m.chercheZone:
		b.WriteString(fmt.Sprintf("  %s résolution de la zone…\n", m.spinner.View()))
	case len(m.zonesTrouvees) == 0:
		b.WriteString("  " + styleDim.Render(
			"Une région, un département ou une commune : « Landes », « Bayonne », "+
				"« Nouvelle-Aquitaine ». ↵ pour chercher.") + "\n")
	default:
		for i, z := range m.zonesTrouvees {
			marque := "[ ]"
			if zoneChoisie(m.formAnn.zones, z) {
				marque = "[×]"
			}
			details := z.Niveau.String() + " · " + z.Source
			if z.CodeInsee != "" {
				details += " · INSEE " + z.CodeInsee
			}
			ligne := fmt.Sprintf("%s %-34s", marque, z.String())
			if i == m.curZoneTrouvee {
				b.WriteString("  " + styleSel.Render(" ▸ "+ligne+" ") + "  " +
					styleDim.Render(details) + "\n")
			} else {
				b.WriteString("    " + ligne + "  " + styleDim.Render(details) + "\n")
			}
		}
	}

	if len(m.formAnn.zones) > 0 {
		noms := make([]string, 0, len(m.formAnn.zones))
		for _, z := range m.formAnn.zones {
			noms = append(noms, z.Nom)
		}
		b.WriteString("\n  " + styleDim.Render("Retenu : "+strings.Join(noms, ", ")) + "\n")
	}
	if m.alerte != "" {
		b.WriteString("\n  " + styleWarn.Render(m.alerte) + "\n")
	}

	b.WriteString("\n  " + styleAide.Render(
		"↵ chercher, puis ↵ retenir · espace cumuler plusieurs zones · esc retour") + "\n")
	b.WriteString("  " + styleDim.Render(
		"Une région est envoyée au portail comme l'union de ses départements.") + "\n")
	return b.String()
}

func (m model) vueResultats() string {
	if m.chercheAnn {
		return fmt.Sprintf("\n  %s interrogation des portails…\n\n  %s\n\n  %s\n",
			m.spinner.View(), styleDim.Render(m.critAnn.Resume()),
			styleAide.Render("esc revenir aux critères · q quitter"))
	}

	var b strings.Builder
	b.WriteString("\n  " + styleTitre.Render(m.titreResultats()) + "\n")
	b.WriteString("  " + styleDim.Render(m.critAnn.Resume()) + "\n\n")

	if m.pretVP {
		b.WriteString(m.vp.View())
	} else {
		b.WriteString(m.contenuAnnonces())
	}

	if m.alerte != "" {
		b.WriteString("\n  " + styleWarn.Render(m.alerte))
	}
	for src, err := range m.erreursAnn {
		b.WriteString("\n  " + styleWarn.Render(src+" : "+err.Error()))
	}
	if m.noteVue != "" {
		b.WriteString("\n  " + styleDim.Render(m.noteVue))
	}

	b.WriteString("\n  " + styleAide.Render(
		"↑/↓ parcourir · s critère de tri · i inverser · ↵ ouvrir l'annonce · "+
			"r relancer · f critères · q quitter"))
	b.WriteString("\n  " + styleDim.Render(m.avertissementEchantillon()) + "\n")
	return b.String()
}

func (m model) titreResultats() string {
	return fmt.Sprintf("Biens en vente (%d) — tri par %s",
		len(m.annonces), m.triAnn.String())
}

// avertissementEchantillon dit ce que le tableau n'est pas.
//
// C'est la ligne la plus importante de l'écran. Trier deux cents annonces par
// prix et afficher « la moins chère » alors que le portail en annonce sept
// mille serait un mensonge par omission — et le genre de mensonge qu'on ne
// remarque jamais soi-même, parce que le tableau, lui, a l'air juste.
func (m model) avertissementEchantillon() string {
	if len(m.annonces) == 0 {
		return "Les prix affichés sont des prix demandés, pas des prix de vente."
	}
	if m.totalAnn > len(m.annonces) {
		return fmt.Sprintf(
			"%d annonces rapportées sur %d annoncées : le tri porte sur cet échantillon, pas sur le marché entier.",
			len(m.annonces), m.totalAnn)
	}
	return "Prix demandés par les vendeurs — à comparer aux ventes DVF, pas à confondre avec elles."
}

// contenuAnnonces construit le tableau. Colonnes fixes et alignées à droite
// pour les nombres : c'est ce qui permet de comparer deux prix d'un coup d'œil
// sans les lire.
func (m model) contenuAnnonces() string {
	var b strings.Builder

	entete := fmt.Sprintf("  %-13s %10s %-12s %8s %9s %5s  %-22s %s",
		"PRIX", "€/M²", "TYPE", "SURFACE", "TERRAIN", "P.", "COMMUNE", "DIST.")
	b.WriteString(styleSection.Render(entete) + "\n")

	for i, a := range m.annonces {
		pieces := ""
		if a.Pieces > 0 {
			pieces = fmt.Sprintf("%d", a.Pieces)
		}
		dist := ""
		if m.critAnn.RayonKm > 0 && a.Localisee() {
			dist = fmt.Sprintf("%.1f km",
				DistanceKm(m.critAnn.CentreLat, m.critAnn.CentreLon, a.Lat, a.Lon))
		}
		commune := a.Commune
		if a.CodePostal != "" {
			commune += " " + a.CodePostal
		}

		ligne := fmt.Sprintf("%-13s %10s %-12s %8s %9s %5s  %-22s %s",
			euros(a.Prix), eurosM2(a.PrixM2()), a.Type.String(),
			m2(a.Surface), m2(a.SurfaceTerrain), pieces,
			tronquer(commune, 22), dist)

		if i == m.curAnnonce {
			b.WriteString(styleSel.Render(" ▸"+ligne+" ") + "\n")
		} else {
			b.WriteString("  " + ligne + "\n")
		}
	}

	if a, ok := m.annonceCourante(); ok {
		b.WriteString("\n  " + styleDim.Render(tronquer(a.Titre, 100)) + "\n")
		details := a.Source
		if a.Neuf {
			details += " · neuf"
		}
		if a.Pro {
			details += " · professionnel"
		}
		if !a.Publiee.IsZero() {
			details += " · publiée le " + a.Publiee.Format("02/01/2006")
		}
		b.WriteString("  " + styleDim.Render(details) + "\n")
		b.WriteString("  " + styleDim.Render(a.URL) + "\n")
	}
	return b.String()
}

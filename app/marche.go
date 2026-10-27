package main

// Onglet « En vente » : les annonces de la commune, chacune confrontée aux
// ventes DVF qui lui ressemblent.
//
// Les annonces ne sont pas chargées avec la commune : l'onglet les demande la
// première fois qu'on l'ouvre. Interroger un portail à chaque consultation de
// statistiques, pour un onglet qu'on n'ouvrira peut-être pas, serait
// discourtois envers lui et lent pour nous.

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const ongletMarche = "En vente"

// limiteMarche : de quoi couvrir toute l'offre d'une commune moyenne. Au-delà,
// on prend les moins chères et on le dit.
const limiteMarche = 500

// etatMarche : les annonces d'une commune, gardées tant qu'on reste sur elle.
// Changer de millésime ne les invalide pas — seule la référence DVF change, et
// elle se recalcule à l'affichage.
type etatMarche struct {
	insee    string
	nom      string
	enCours  bool
	err      error
	zones    []ZoneAnnonce
	annonces []Annonce
	total    int
	erreurs  map[string]error
	annuler  context.CancelFunc
}

type msgMarche struct {
	insee string
	zones []ZoneAnnonce
	res   ResultatAnnonces
	err   error
}

// zonesDeCommune ne garde que les zones qui désignent exactement la commune.
// Le nom seul ne suffit pas : il existe une vingtaine de Saint-Martin, et
// l'autocomplétion classe par pertinence, pas par code. Le code INSEE est le
// seul pont fiable entre DVF et un portail — une zone qui ne le donne pas ne
// peut pas être vérifiée, elle est écartée.
func zonesDeCommune(zones []ZoneAnnonce, insee string) []ZoneAnnonce {
	var out []ZoneAnnonce
	vues := map[string]bool{}
	for _, z := range zones {
		if z.Niveau != ZoneCommune || z.CodeInsee != insee || vues[z.Source] {
			continue
		}
		vues[z.Source] = true // une zone par portail suffit
		out = append(out, z)
	}
	return out
}

// criteresMarche : tous les biens anciens de la commune. Le neuf est exclu
// parce que DVF l'est aussi en pratique : une vente sur plan est une « vente
// en l'état futur d'achèvement », que la consolidation écarte. Le comparer à
// de l'ancien mesurerait la prime du neuf, pas le marché.
func criteresMarche(zones []ZoneAnnonce) Criteres {
	return Criteres{
		Zones:       zones,
		Types:       TousTypes(),
		ExclureNeuf: true,
		Tri:         TriAnnonces{Critere: TriAPrix},
		Limite:      limiteMarche,
	}
}

func cmdMarche(ctx context.Context, insee, nom string) tea.Cmd {
	return func() tea.Msg {
		zones, err := ResoudreZones(ctx, nom)
		if err != nil {
			return msgMarche{insee: insee, err: err}
		}
		zones = zonesDeCommune(zones, insee)
		if len(zones) == 0 {
			return msgMarche{insee: insee, err: fmt.Errorf(
				"aucun portail ne connaît la commune %s (%s)", nom, insee)}
		}
		res, err := ChercherAnnonces(ctx, criteresMarche(zones))
		return msgMarche{insee: insee, zones: zones, res: res, err: err}
	}
}

// chargerMarche lance la recherche si l'onglet est affiché et que les annonces
// en mémoire ne sont pas celles de la commune.
func (m *model) chargerMarche() tea.Cmd {
	if m.onglets[m.curOnglet] != ongletMarche {
		return nil
	}
	insee, nom := CommuneDominante(m.data.Sales)
	if insee == "" || (insee == m.marche.insee && (m.marche.enCours || m.marche.err == nil)) {
		return nil
	}
	if m.marche.annuler != nil {
		m.marche.annuler()
	}
	if m.commune.Code == insee && m.commune.Nom != "" {
		nom = m.commune.Nom // le nom officiel, plutôt que celui d'une ligne DVF
	}
	ctx, annuler := context.WithCancel(context.Background())
	m.marche = etatMarche{insee: insee, nom: nom, enCours: true, annuler: annuler}
	return cmdMarche(ctx, insee, nom)
}

func (m model) appliquerMarche(msg msgMarche) (tea.Model, tea.Cmd) {
	if msg.insee != m.marche.insee {
		return m, nil // réponse pour une commune qu'on a quittée
	}
	m.marche.enCours = false
	m.marche.err = msg.err
	m.marche.zones = msg.zones
	m.marche.annonces = msg.res.Annonces
	m.marche.total = msg.res.Total
	m.marche.erreurs = msg.res.Erreurs
	m.curMarche = 0
	if err := m.suivi.Croiser(m.marche.annonces, time.Now()); err != nil {
		m.noteVue = err.Error()
	}
	if m.ecran == ecranStats && m.onglets[m.curOnglet] == ongletMarche {
		m.majStats()
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Rendu
// ---------------------------------------------------------------------------

// vueMarche a un récepteur pointeur : elle note, en écrivant, la ligne de
// chaque annonce, pour le défilement et le clic.
func (m *model) vueMarche() string {
	var b strings.Builder
	b.WriteString("\n")

	insee, _ := CommuneDominante(m.data.Sales)
	switch {
	case insee == "":
		b.WriteString("  " + styleWarn.Render("Code commune introuvable dans les ventes : impossible d'interroger les portails.") + "\n")
		return b.String()
	case insee != m.marche.insee || m.marche.enCours:
		// Pas de spinner : le contenu du viewport n'est recalculé qu'à
		// l'arrivée de la réponse, une animation y resterait figée.
		b.WriteString("  " + styleDim.Render("Recherche des biens en vente sur les portails…") + "\n")
		return b.String()
	case m.marche.err != nil:
		b.WriteString("  " + styleWarn.Render("Erreur : "+m.marche.err.Error()) + "\n")
		b.WriteString("  " + styleDim.Render("Repasse sur cet onglet pour réessayer.") + "\n")
		return b.String()
	}

	as := m.marche.annonces
	var portails []string
	for _, z := range m.marche.zones {
		portails = append(portails, z.Source)
	}
	b.WriteString("  " + styleSection.Render(fmt.Sprintf("BIENS ANCIENS EN VENTE — %s", strings.ToUpper(m.marche.nom))) + "\n\n")
	entete := fmt.Sprintf("  %s sur %s", pluriel(len(as), "annonce"), strings.Join(portails, ", "))
	if m.marche.total > len(as) {
		entete += fmt.Sprintf(" (les moins chères sur %d annoncées)", m.marche.total)
	}
	b.WriteString(entete + "\n")
	if nouvelles, vues := m.suivi.Compter(as); nouvelles+vues > 0 {
		b.WriteString("  " + styleDim.Render(fmt.Sprintf(
			"%s (●) depuis le lancement, %s (✓) · m pour marquer vue ou non vue",
			pluriel(nouvelles, "nouvelle"), pluriel(vues, "déjà vue"))) + "\n")
	}
	for nom, err := range m.marche.erreurs {
		b.WriteString("  " + styleWarn.Render(nom+" : "+err.Error()) + "\n")
	}
	lignes, viagers, doublons := m.lignesMarche()
	m.lignesAnnonces = make([]int, len(lignes))
	if viagers > 0 || doublons > 0 {
		var ecartees []string
		if viagers > 0 {
			ecartees = append(ecartees, pluriel(viagers, "viager")+" (le prix n'est que le bouquet)")
		}
		if doublons > 0 {
			ecartees = append(ecartees, pluriel(doublons, "doublon")+" (même bien chez plusieurs agences)")
		}
		b.WriteString("  " + styleDim.Render("Écartés : "+strings.Join(ecartees, ", ")+".") + "\n")
	}
	b.WriteString("  " + styleDim.Render(fmt.Sprintf("Comparées à %s DVF de la période %s.",
		pluriel(len(m.data.Sales), "vente"), m.data.Annee)) + "\n")
	if len(lignes) == 0 {
		return b.String()
	}

	b.WriteString("\n  " + styleSection.Render("PRIX DEMANDÉ CONTRE PRIX DE VENTE DE BIENS SIMILAIRES") + "\n\n")
	b.WriteString(styleDim.Render(fmt.Sprintf("  %-12s %8s %9s %14s %14s %11s %13s",
		"Type", "Annonces", "Comparées", "€/m² demandé", "€/m² vendu", "Écart méd.", "Plus chères")) + "\n")
	synth := Synthetiser(lignes)
	for _, s := range synth {
		plus := "–"
		if s.Comparees > 0 {
			plus = fmt.Sprintf("%d / %d", s.AuDessus, s.Comparees)
		}
		b.WriteString(fmt.Sprintf("  %-12s %8d %9d %14s %14s %11s %13s\n",
			s.Type, s.Annonces, s.Comparees, eurosM2(s.PrixM2Ann), eurosM2(s.PrixM2Vendu),
			pourcent(s.EcartMed, s.Comparees > 0), plus))
	}
	for _, s := range synth {
		if s.Comparees >= minSimilaires {
			b.WriteString("\n  " + lectureEcart(s) + "\n")
		}
	}

	for _, t := range TousTypes() {
		m.ecrireComparaisons(&b, t, lignes)
	}

	b.WriteString("\n  " + styleSection.Render("MÉTHODE ET LIMITES") + "\n\n")
	for _, l := range []string{
		"Chaque annonce est comparée à ses propres ventes similaires, jamais à la médiane de la commune :",
		"si l'offre du moment compte surtout de grands biens, une comparaison globale mesurerait cela.",
		fmt.Sprintf("Similaire : même type, surface à ±%.0f %%, même nombre de pièces, terrain entre ½ et 2 fois.",
			criteresSimilarite[0].surface*100),
		fmt.Sprintf("À défaut de %d ventes, surface à ±%.0f %%, ±1 pièce, terrain entre ⅓ et 3 fois (marqué ≈).",
			minSimilaires, criteresSimilarite[1].surface*100),
		"Le prix comparé est celui qui figurera dans l'acte : hors honoraires d'agence quand l'acheteur",
		"les paie, comme dans DVF. Il reste un prix demandé, que la négociation fera souvent baisser :",
		"un écart de quelques pour cent au-dessus des ventes est donc la norme, pas une anomalie.",
		"Les ventes de référence couvrent toute la période choisie : sur plusieurs millésimes, l'écart",
		"inclut aussi l'évolution des prix depuis. Choisir une seule année l'atténue, au prix d'effectifs plus faibles.",
	} {
		b.WriteString("  " + styleDim.Render(l) + "\n")
	}
	return b.String()
}

// lectureEcart traduit la synthèse en une phrase.
func lectureEcart(s SyntheseMarche) string {
	phrase := fmt.Sprintf("%s : les vendeurs demandent en médiane %.0f %% de plus que le prix signé pour des biens similaires",
		s.Type, s.EcartMed)
	switch {
	case s.EcartMed > -0.5 && s.EcartMed < 0.5:
		phrase = fmt.Sprintf("%s : les vendeurs demandent en médiane le prix signé pour des biens similaires", s.Type)
	case s.EcartMed < 0:
		phrase = fmt.Sprintf("%s : les vendeurs demandent en médiane %.0f %% de moins que le prix signé pour des biens similaires",
			s.Type, -s.EcartMed)
	}
	return phrase +
		styleDim.Render(fmt.Sprintf(" (la moitié des annonces entre %s et %s).",
			pourcent(s.EcartQ1, true), pourcent(s.EcartQ3, true)))
}

// lignesMarche renvoie les annonces dans l'ordre exact de l'affichage : par
// type de bien, les comparées de la plus avantageuse à la plus chère, puis les
// autres. Le curseur de l'onglet est un rang dans cette liste, et l'affichage
// comme la navigation la recalculent de la même façon : ils ne peuvent pas
// diverger.
func (m model) lignesMarche() (lignes []Comparaison, viagers, doublons int) {
	insee, _ := CommuneDominante(m.data.Sales)
	if insee == "" || insee != m.marche.insee || m.marche.enCours || m.marche.err != nil {
		return nil, 0, 0
	}
	as, viagers, doublons := PreparerMarche(m.marche.annonces)
	cs := ComparerTout(as, m.data.Sales)
	for _, t := range TousTypes() {
		for _, c := range cs {
			if c.Annonce.Type == t {
				lignes = append(lignes, c)
			}
		}
	}
	return lignes, viagers, doublons
}

// annonceMarche renvoie l'annonce sous le curseur de l'onglet.
func (m model) annonceMarche() (Annonce, bool) {
	lignes, _, _ := m.lignesMarche()
	if m.curMarche < 0 || m.curMarche >= len(lignes) {
		return Annonce{}, false
	}
	return lignes[m.curMarche].Annonce, true
}

func (m model) deplacerMarche(pas int) (tea.Model, tea.Cmd) {
	lignes, _, _ := m.lignesMarche()
	if len(lignes) == 0 {
		return m, nil
	}
	m.curMarche = borner(m.curMarche+pas, len(lignes))
	m.noteVue = ""
	m.majStats()
	return m, nil
}

func (m model) ouvrirAnnonceMarche() (tea.Model, tea.Cmd) {
	a, ok := m.annonceMarche()
	if !ok || a.URL == "" {
		return m, nil
	}
	m.noteVue = "Ouverture de l'annonce…"
	if err := m.suivi.MarquerVue(a, time.Now()); err != nil {
		m.noteVue = err.Error()
	}
	m.majStats()
	return m, cmdOuvrirVue(a.URL)
}

// basculerAnnonceMarche : la touche « m » de l'onglet En vente.
func (m model) basculerAnnonceMarche() (tea.Model, tea.Cmd) {
	a, ok := m.annonceMarche()
	if !ok {
		return m, nil
	}
	m.noteVue = noteBascule(m.suivi.Basculer(a, time.Now()))
	m.majStats()
	return m, nil
}

// annonceSous renvoie le rang de l'annonce affichée à la ligne écran y. Les
// lignes du tableau ne sont pas contiguës (une section par type de bien) :
// on cherche la ligne notée à l'écriture plutôt que de la calculer.
func (m model) annonceSous(y int) int {
	ligne := m.vp.YOffset + (y - entetesStats)
	for i, l := range m.lignesAnnonces {
		if l == ligne {
			return i
		}
	}
	return -1
}

// ecrireComparaisons écrit les annonces d'un type. lignes est la liste
// complète de l'onglet : le rang d'une annonce y est celui du curseur.
func (m *model) ecrireComparaisons(b *strings.Builder, t TypeBien, lignes []Comparaison) {
	premiere, sans := true, 0
	for i, c := range lignes {
		if c.Annonce.Type != t {
			continue
		}
		if premiere {
			premiere = false
			b.WriteString("\n  " + styleSection.Render(strings.ToUpper(t.String())) + "\n\n")
			if t == BienTerrain {
				b.WriteString(styleDim.Render(fmt.Sprintf("    %9s %13s %13s %8s %11s   %s",
					"Terrain", "Prix acte", "Estimé DVF", "Écart", "Similaires", "Annonce")) + "\n")
			} else {
				b.WriteString(styleDim.Render(fmt.Sprintf("    %8s %3s %9s %13s %13s %8s %11s   %s",
					"Surface", "P.", "Terrain", "Prix acte", "Estimé DVF", "Écart", "Similaires", "Annonce")) + "\n")
			}
		}
		if !c.Comparable() {
			sans++
		}

		ligne := strings.Count(b.String(), "\n")
		m.lignesAnnonces[i] = ligne
		marque := m.suivi.Marque(c.Annonce) + " "
		switch {
		case i == m.curMarche:
			// Une ligne sélectionnée est rendue d'un seul style : des couleurs
			// imbriquées couperaient la surbrillance à chaque séquence de fin.
			m.ligneSel = ligne
			b.WriteString(styleSel.Render("▸ "+marque+ligneComparaison(c, false)) + "\n")
		case m.suivi.Vue(c.Annonce):
			b.WriteString("  " + styleDim.Render(marque+ligneComparaison(c, false)) + "\n")
		default:
			b.WriteString("  " + marque + ligneComparaison(c, true) + "\n")
		}
	}
	if sans > 0 {
		b.WriteString("  " + styleDim.Render(fmt.Sprintf(
			"%s sans comparaison (–) : surface absente, ou moins de %d ventes similaires sur la période.",
			pluriel(sans, "annonce"), minSimilaires)) + "\n")
	}
}

// ligneComparaison met en forme une annonce. Sans couleur pour la ligne
// sélectionnée, qui porte son propre style.
func ligneComparaison(c Comparaison, couleur bool) string {
	a := c.Annonce
	estime, sim := "–", "–"
	ecart := fmt.Sprintf("%8s", "–")
	if c.Comparable() {
		estime = euros(c.Estime)
		sim = fmt.Sprintf("%d", len(c.Similaires))
		if c.Niveau == SimilariteElargie {
			sim = "≈ " + sim
		}
		ecart = fmt.Sprintf("%8s", pourcent(c.Ecart, true))
		switch {
		case couleur && c.Ecart <= -10:
			ecart = styleOK.Render(ecart)
		case couleur && c.Ecart >= 10:
			ecart = styleWarn.Render(ecart)
		}
	}
	titre := tronquer(a.Titre, 40)
	if couleur {
		titre = styleDim.Render(titre)
	}
	if a.Type == BienTerrain {
		return fmt.Sprintf("%9s %13s %13s %s %11s   %s",
			m2(a.SurfaceTerrain), euros(a.PrixCompare()), estime, ecart, sim, titre)
	}
	pieces := "–"
	if a.Pieces > 0 {
		pieces = fmt.Sprintf("%d", a.Pieces)
	}
	return fmt.Sprintf("%8s %3s %9s %13s %13s %s %11s   %s",
		m2(a.Surface), pieces, m2(a.SurfaceTerrain), euros(a.PrixCompare()), estime, ecart, sim, titre)
}

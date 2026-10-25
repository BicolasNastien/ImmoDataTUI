package main

// Tableau des ventes et fiche détaillée d'une vente.
//
// Les statistiques répondent à « combien vaut un bien ici ? ». Le tableau et la
// fiche répondent à la question suivante, tout aussi concrète : « quelles sont
// exactement les ventes derrière ces médianes ? ». Rien n'y est agrégé — la
// fiche restitue la totalité des colonnes DVF de la mutation.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Sélection courante
// ---------------------------------------------------------------------------

// typeOnglet renvoie le type de bien listé par l'onglet courant. Vide : soit
// l'onglet n'a pas de tableau, soit il les liste tous.
func (m model) typeOnglet() string {
	switch m.onglets[m.curOnglet] {
	case "Maisons":
		return "Maison"
	case "Appartements":
		return "Appartement"
	case "Terrains":
		return "Terrain"
	}
	return ""
}

// jeu renvoie le jeu de données que l'onglet courant décrit : le millésime
// choisi, ou tous les millésimes fusionnés. Le reste de l'affichage n'a pas à
// savoir lequel des deux il manipule.
func (m model) jeu() Dataset {
	if m.onglets[m.curOnglet] == "Adresse" {
		return m.total
	}
	return m.data
}

// ventesOnglet renvoie les ventes listées par l'onglet courant, dans l'ordre
// du tri actif.
func (m model) ventesOnglet() []Sale {
	switch m.onglets[m.curOnglet] {
	case "Résumé", ongletMarche:
		return nil
	case "Adresse":
		return m.ventesAdresse()
	}
	return VentesTriees(m.data.Sales, m.typeOnglet(), m.tri)
}

// medianesM2 renvoie le €/m² médian par type de bien. Il sert de référence à la
// colonne d'écart du tableau : comparer un appartement à la médiane des maisons
// n'aurait aucun sens.
func medianesM2(sales []Sale) map[string]float64 {
	par := map[string][]float64{}
	for _, s := range sales {
		if v := s.PrixM2Ref(); v > 0 {
			par[s.Type] = append(par[s.Type], v)
		}
	}
	out := map[string]float64{}
	for t, v := range par {
		out[t] = mediane(v)
	}
	return out
}

// venteCourante renvoie la vente sous le curseur.
func (m model) venteCourante() (Sale, bool) {
	v := m.ventesOnglet()
	if m.curVente < 0 || m.curVente >= len(v) {
		return Sale{}, false
	}
	return v[m.curVente], true
}

// changerOnglet passe à l'onglet suivant ou précédent : le curseur de vente
// repart du début, le tableau n'étant pas le même.
func (m *model) changerOnglet(pas int) {
	m.curOnglet = (m.curOnglet + pas + len(m.onglets)) % len(m.onglets)
	m.curVente = 0
	m.curMarche = 0
	m.noteVue = ""
	m.majStats()
	m.vp.GotoTop()
}

// deplacerVente déplace le curseur dans le tableau et réaffiche l'écran
// concerné : le tableau ou la fiche, selon là où l'on se trouve.
func (m model) deplacerVente(pas int) (tea.Model, tea.Cmd) {
	n := len(m.ventesOnglet())
	if n == 0 {
		return m, nil
	}
	m.curVente = borner(m.curVente+pas, n)
	m.noteVue = ""
	if !m.pretVP {
		return m, nil
	}
	if m.ecran == ecranVente {
		m.vp.SetContent(m.contenuVente())
		m.vp.GotoTop()
		return m, nil
	}
	m.majStats()
	return m, nil
}

// majStats recalcule le contenu de l'écran statistiques et fait suivre le
// défilement à la ligne sélectionnée.
func (m *model) majStats() {
	if !m.pretVP {
		return
	}
	m.ligneSel, m.ligneTableau = -1, -1
	m.vp.SetContent(m.contenuStats())
	m.suivreLigne()
}

// suivreLigne fait défiler le viewport juste ce qu'il faut pour que la ligne
// sélectionnée reste visible, avec une ligne de marge.
func (m *model) suivreLigne() {
	if m.ligneSel < 0 {
		return
	}
	haut := m.vp.YOffset
	bas := haut + m.vp.Height - 1
	switch {
	case m.ligneSel < haut+1:
		m.vp.SetYOffset(m.ligneSel - 1)
	case m.ligneSel > bas-1:
		m.vp.SetYOffset(m.ligneSel - m.vp.Height + 2)
	}
}

// ---------------------------------------------------------------------------
// Tableau des ventes
// ---------------------------------------------------------------------------

// tronquer coupe une chaîne à n caractères (et non à n octets : les noms de
// voie sont accentués).
func tronquer(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// pourcent formate un écart relatif signé : « +12 % », « -8 % ».
func pourcent(v float64, ok bool) string {
	if !ok {
		return "–"
	}
	// Arrondi d'abord : -0,4 s'afficherait « -0 % », un signe qui ne dit rien.
	if v > -0.5 && v < 0.5 {
		return "0 %"
	}
	return fmt.Sprintf("%+.0f %%", v)
}

// colonnes décrit le jeu de colonnes d'un tableau : il dépend de ce qu'on liste.
type colonnes int

const (
	colBati    colonnes = iota // Maisons, Appartements : surface bâtie et pièces
	colTerrain                 // Terrains nus : surface de terrain et parcelles
	colMixte                   // tous types confondus : une colonne Type
)

func (m model) colonnesOnglet() colonnes {
	switch m.typeOnglet() {
	case "Terrain":
		return colTerrain
	case "":
		return colMixte
	}
	return colBati
}

// ecrireTableau ajoute le tableau des ventes de l'onglet. La colonne d'écart se
// lit par rapport à la médiane du type de la vente : c'est ce qui situe chaque
// ligne dans son marché, ce qu'aucune moyenne ne dit.
func (m *model) ecrireTableau(b *strings.Builder, ventes []Sale) {
	if len(ventes) == 0 {
		return
	}
	cols := m.colonnesOnglet()
	refs := medianesM2(m.jeu().Sales)

	titre := fmt.Sprintf("VENTES (%d) — tri par %s", len(ventes), LibellesTri[m.tri])
	b.WriteString("\n  " + styleSection.Render(titre) + "\n\n")

	// Largeur restante pour l'adresse : le reste du tableau est à largeur fixe.
	largeurFixe := map[colonnes]int{colBati: 68, colTerrain: 63, colMixte: 68}[cols]
	largeurVoie := m.largeur - largeurFixe
	if largeurVoie < 16 {
		largeurVoie = 16
	}

	var entete string
	switch cols {
	case colTerrain:
		entete = fmt.Sprintf("  %-10s %13s %10s %12s %8s %5s  %s",
			"Date", "Prix", "Terrain", "€/m²", "vs méd.", "Parc.", "Adresse")
	case colMixte:
		entete = fmt.Sprintf("  %-10s %-13s %13s %8s %12s %8s  %s",
			"Date", "Type", "Prix", "Surface", "€/m²", "vs méd.", "Adresse")
	default:
		entete = fmt.Sprintf("  %-10s %13s %8s %3s %12s %8s %9s  %s",
			"Date", "Prix", "Surface", "P.", "€/m²", "vs méd.", "Terrain", "Adresse")
	}
	b.WriteString(styleDim.Render(entete) + "\n")

	base := strings.Count(b.String(), "\n")
	m.ligneTableau = base
	for i, v := range ventes {
		ecart, ok := EcartRelatif(v.PrixM2Ref(), refs[v.Type])
		adresse := tronquer(v.Adresse(), largeurVoie)
		date := v.Date.Format("02/01/2006")

		var ligne string
		switch cols {
		case colTerrain:
			ligne = fmt.Sprintf("%-10s %13s %10s %12s %8s %5d  %s",
				date, euros(v.Valeur), m2(v.SurfaceTerrain),
				eurosM2(v.PrixM2Terrain()), pourcent(ecart, ok), len(v.Parcelles()), adresse)
		case colMixte:
			ligne = fmt.Sprintf("%-10s %-13s %13s %8s %12s %8s  %s",
				date, tronquer(typeCourt(v.Type), 13), euros(v.Valeur), m2(v.SurfaceRef()),
				eurosM2(v.PrixM2Ref()), pourcent(ecart, ok), adresse)
		default:
			pieces := "–"
			if v.Pieces > 0 {
				pieces = fmt.Sprintf("%d", v.Pieces)
			}
			ligne = fmt.Sprintf("%-10s %13s %8s %3s %12s %8s %9s  %s",
				date, euros(v.Valeur), m2(v.SurfaceBati), pieces,
				eurosM2(v.PrixM2()), pourcent(ecart, ok), m2(v.SurfaceTerrain), adresse)
		}

		if i == m.curVente {
			m.ligneSel = base + i
			b.WriteString(styleSel.Render("▸ "+ligne) + "\n")
		} else {
			b.WriteString("  " + ligne + "\n")
		}
	}

	b.WriteString("\n  " + styleDim.Render(
		"« vs méd. » situe le €/m² de la vente par rapport à la médiane de son type de bien.") + "\n")
}

// typeCourt abrège le seul libellé DVF qui ne tient pas dans une colonne.
func typeCourt(t string) string {
	if t == "Local industriel. commercial ou assimilé" {
		return "Local pro."
	}
	return t
}

// ---------------------------------------------------------------------------
// Fiche détaillée
// ---------------------------------------------------------------------------

func (m model) vueVente() string {
	var b strings.Builder

	v, ok := m.venteCourante()
	titre := "Vente"
	if ok {
		titre = fmt.Sprintf("%s — %s", v.Type, v.Adresse())
	}
	b.WriteString("\n  " + styleTitre.Render(titre) + "\n")
	b.WriteString("  " + styleDim.Render(fmt.Sprintf("vente %d / %d — %s %s",
		m.curVente+1, len(m.ventesOnglet()), m.data.Ville, m.data.Annee)) + "\n")

	if m.pretVP {
		b.WriteString(m.vp.View())
	} else {
		b.WriteString(m.contenuVente())
	}

	b.WriteString("\n  " + styleAide.Render(
		"v carte · c cadastre · V vue de la rue · n/p vente suiv./préc. · ↑/↓ défiler · esc retour"))
	return b.String()
}

// champ écrit une ligne « libellé : valeur », en sautant les valeurs vides —
// une fiche criblée de tirets se lit mal.
func champ(b *strings.Builder, libelle, valeur string) {
	if strings.TrimSpace(valeur) == "" || valeur == "–" {
		return
	}
	b.WriteString(fmt.Sprintf("  %-26s %s\n", libelle, valeur))
}

func section(b *strings.Builder, titre string) {
	b.WriteString("\n  " + styleSection.Render(titre) + "\n\n")
}

func (m model) contenuVente() string {
	v, ok := m.venteCourante()
	if !ok {
		return "\n  " + styleWarn.Render("Vente introuvable.") + "\n"
	}
	ref := v.Lignes[0]
	var b strings.Builder

	// --- Prix -------------------------------------------------------------
	section(&b, "PRIX")
	champ(&b, "Valeur foncière", styleOK.Render(euros(v.Valeur)))
	if v.SurfaceBati > 0 {
		champ(&b, "Prix au m² bâti", eurosM2(v.PrixM2()))
	}
	if v.SurfaceTerrain > 0 {
		champ(&b, "Prix au m² de terrain", eurosM2(v.PrixM2Terrain()))
	}

	// Situer la vente dans son marché : c'est ce qui transforme un chiffre
	// isolé en information exploitable.
	st := StatsPourType(m.data.Sales, v.Type)
	medPrix, medM2 := st.Prix.Med, st.PrixM2.Med
	if v.Type == "Terrain" {
		p, pm2 := StatsTerrain(m.data.Sales)
		medPrix, medM2 = p.Med, pm2.Med
	}
	if e, ok := EcartRelatif(v.Valeur, medPrix); ok {
		champ(&b, "Écart au prix médian", fmt.Sprintf("%s   %s",
			pourcent(e, true), styleDim.Render("médiane "+euros(medPrix))))
	}
	if e, ok := EcartRelatif(v.PrixM2Ref(), medM2); ok {
		champ(&b, "Écart au €/m² médian", fmt.Sprintf("%s   %s",
			pourcent(e, true), styleDim.Render("médiane "+eurosM2(medM2))))
	}

	// --- Bien -------------------------------------------------------------
	section(&b, "BIEN")
	champ(&b, "Type", v.Type)
	champ(&b, "Code type local", ref.CodeTypeLocal)
	champ(&b, "Surface réelle bâtie", m2(v.SurfaceBati))
	if v.Pieces > 0 {
		champ(&b, "Pièces principales", fmt.Sprintf("%d", v.Pieces))
	}
	champ(&b, "Surface de terrain", m2(v.SurfaceTerrain))
	if ref.NombreLots > 0 {
		champ(&b, "Nombre de lots", fmt.Sprintf("%d", ref.NombreLots))
	}
	champ(&b, "Numéro de volume", ref.NumeroVolume)
	for _, l := range v.LotsCopro() {
		valeur := "lot " + l.Numero
		if l.Carrez > 0 {
			valeur += fmt.Sprintf("  —  %s Carrez", m2(l.Carrez))
		}
		champ(&b, "Lot de copropriété", valeur)
	}

	// Plusieurs locaux dans la mutation : dépendances, annexes, caves.
	if locaux := v.Locaux(); len(locaux) > 1 {
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  Locaux de la mutation") + "\n")
		for _, l := range locaux {
			pieces := ""
			if l.Pieces > 0 {
				pieces = fmt.Sprintf(", %d pièces", l.Pieces)
			}
			b.WriteString(fmt.Sprintf("    · %s — %s%s\n", l.TypeLocal, m2(l.SurfaceBati), pieces))
		}
	}

	// --- Adresse ----------------------------------------------------------
	section(&b, "ADRESSE")
	champ(&b, "Adresse", v.Adresse())
	champ(&b, "Commune", v.Ville())
	champ(&b, "Code commune (INSEE)", ref.CodeCommune)
	champ(&b, "Département", ref.CodeDepartement)
	champ(&b, "Code voie (FANTOIR)", ref.CodeVoie)
	champ(&b, "Ancienne commune", strings.TrimSpace(ref.AncienCodeCommune+" "+ref.AncienNomCommune))
	if lat, lon, ok := v.Position(); ok {
		champ(&b, "Coordonnées", fmt.Sprintf("%.5f, %.5f", lat, lon))
	}

	// --- Mutation ---------------------------------------------------------
	section(&b, "MUTATION")
	champ(&b, "Identifiant", v.ID)
	champ(&b, "Date", v.Date.Format("02/01/2006"))
	champ(&b, "Nature", ref.Nature)
	champ(&b, "Numéro de disposition", ref.NumeroDisposition)
	champ(&b, "Lignes dans le fichier", fmt.Sprintf("%d", len(v.Lignes)))

	// --- Parcelles --------------------------------------------------------
	if parcelles := v.Parcelles(); len(parcelles) > 0 {
		section(&b, "PARCELLES ET NATURES DE CULTURE")
		b.WriteString(styleDim.Render("  Parcelle              Nature de culture             Surface") + "\n")
		for _, p := range parcelles {
			b.WriteString(fmt.Sprintf("  %-21s %-29s %9s\n",
				p.Parcelle, tronquer(p.Culture, 29), m2(p.Surface)))
		}
		b.WriteString(fmt.Sprintf("  %-21s %-29s %9s\n", "", styleDim.Render("total"), m2(v.SurfaceTerrain)))
	}
	if ref.AncienIDParcelle != "" {
		champ(&b, "Ancienne parcelle", ref.AncienIDParcelle)
	}

	// --- Lignes brutes ----------------------------------------------------
	// Le dernier recours quand un chiffre surprend : voir le fichier tel quel.
	section(&b, fmt.Sprintf("LIGNES BRUTES DU FICHIER (%d)", len(v.Lignes)))
	b.WriteString(styleDim.Render("  Parcelle              Type local         Bâti   Culture        Terrain") + "\n")
	for _, r := range v.Lignes {
		culture := r.NatureCulture
		if culture == "" {
			culture = r.CodeNatureCulture
		}
		b.WriteString(fmt.Sprintf("  %-21s %-17s %7s   %-14s %7s\n",
			r.IDParcelle, tronquer(r.TypeLocal, 17), m2(r.SurfaceBati),
			tronquer(culture, 14), m2(r.SurfaceTerrain)))
	}

	// --- Voir le bien -----------------------------------------------------
	// Trois vues plutôt qu'une : la vue de la rue montre une façade sans dire
	// laquelle, la carte et le cadastre désignent le bien.
	section(&b, "VOIR LE BIEN")
	_, _, geolocalise := v.Position()
	for _, vue := range VuesExternes {
		if vue.URL(v) == "" {
			continue
		}
		// Les adresses web sont longues et illisibles : on n'affiche que ce que
		// chaque vue apporte. Le lien complet n'apparaît qu'en cas d'échec de
		// l'ouverture, là où il devient utile à copier.
		b.WriteString(fmt.Sprintf("  %s  %-42s %s\n",
			styleOK.Render(vue.Touche), vue.Nom, styleDim.Render(vue.Detail)))
	}
	if !geolocalise {
		b.WriteString("  " + styleWarn.Render(
			"Parcelle non géolocalisée : les liens portent sur l'adresse, pas sur le point exact.") + "\n")
	}
	if m.noteVue != "" {
		b.WriteString("\n  " + styleOK.Render(m.noteVue) + "\n")
	}

	b.WriteString("\n  " + styleDim.Render(
		"Valeur foncière DVF : prix net vendeur, hors frais de notaire et mobilier.") + "\n")
	return b.String()
}

// ---------------------------------------------------------------------------
// Souris dans le tableau
// ---------------------------------------------------------------------------

// entetesStats : lignes affichées au-dessus du viewport (vide, titre, onglets).
const entetesStats = 3

// venteSous renvoie l'indice de la vente affichée à la ligne écran y.
func (m model) venteSous(y int) int {
	if m.ligneTableau < 0 {
		return -1
	}
	i := m.vp.YOffset + (y - entetesStats) - m.ligneTableau
	if i < 0 || i >= len(m.ventesOnglet()) {
		return -1
	}
	return i
}

// sourisTableau gère molette et clic sur l'écran des statistiques et sur la
// fiche. Un clic sélectionne la ligne ; un second clic sur la ligne déjà
// sélectionnée ouvre la fiche — le même geste que sur la carte.
func (m model) sourisTableau(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		m.vp.LineUp(3)
		return m, nil
	case msg.Button == tea.MouseButtonWheelDown:
		m.vp.LineDown(3)
		return m, nil
	case m.ecran == ecranStats && m.onglets[m.curOnglet] == ongletMarche &&
		msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		i := m.annonceSous(msg.Y)
		if i < 0 {
			return m, nil
		}
		if i == m.curMarche {
			return m.ouvrirAnnonceMarche()
		}
		m.curMarche = i
		m.majStats()
		return m, nil
	case m.ecran == ecranStats && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		i := m.venteSous(msg.Y)
		if i < 0 {
			return m, nil
		}
		if i == m.curVente {
			return m.ouvrir()
		}
		m.curVente = i
		m.majStats()
		return m, nil
	}
	return m, nil
}

package main

// Recherche d'adresse (Base Adresse Nationale) et page d'une adresse.
//
// Chercher « 12 rue des Lilas, Troyes » depuis la carte de France évite les
// trois échelons de navigation quand on sait déjà où l'on va. La réponse de la
// BAN porte le code INSEE de la commune : de là, le département se déduit, les
// millésimes se téléchargent, et la page de l'adresse agrège tout ce que DVF
// sait de cette voie et de ce numéro.

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const apiAdresse = "https://api-adresse.data.gouv.fr/search/"

// Adresse est un résultat de géocodage.
type Adresse struct {
	Label      string
	Numero     string
	Voie       string
	CodePostal string
	Commune    string
	CodeInsee  string
	Contexte   string
	Type       string // housenumber, street, locality, municipality
	Lat, Lon   float64
}

// Departement déduit le code département du code INSEE de la commune :
// deux caractères, sauf outre-mer où il en faut trois. La Corse s'en tire
// seule, ses codes commençant par « 2A » et « 2B ».
func (a Adresse) Departement() string {
	if len(a.CodeInsee) < 2 {
		return ""
	}
	if strings.HasPrefix(a.CodeInsee, "97") || strings.HasPrefix(a.CodeInsee, "98") {
		if len(a.CodeInsee) >= 3 {
			return a.CodeInsee[:3]
		}
	}
	return a.CodeInsee[:2]
}

// EstPrecise indique qu'on tient un numéro dans une voie, et pas seulement une
// commune : la page d'adresse n'a pas le même contenu dans les deux cas.
func (a Adresse) EstPrecise() bool { return a.Type == "housenumber" }

// reponseBAN est la forme GeoJSON renvoyée par la BAN.
type reponseBAN struct {
	Features []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			Label       string `json:"label"`
			Housenumber string `json:"housenumber"`
			Street      string `json:"street"`
			Name        string `json:"name"`
			Postcode    string `json:"postcode"`
			City        string `json:"city"`
			Citycode    string `json:"citycode"`
			Context     string `json:"context"`
			Type        string `json:"type"`
		} `json:"properties"`
	} `json:"features"`
}

// parserAdresses décode une réponse BAN. Séparée de la requête : c'est le
// décodage qui mérite un test, pas le réseau.
func parserAdresses(brut []byte) ([]Adresse, error) {
	var r reponseBAN
	if err := json.Unmarshal(brut, &r); err != nil {
		return nil, fmt.Errorf("réponse de la BAN illisible : %w", err)
	}
	var out []Adresse
	for _, f := range r.Features {
		p := f.Properties
		voie := p.Street
		if voie == "" {
			voie = p.Name
		}
		a := Adresse{
			Label: p.Label, Numero: p.Housenumber, Voie: voie,
			CodePostal: p.Postcode, Commune: p.City, CodeInsee: p.Citycode,
			Contexte: p.Context, Type: p.Type,
		}
		if len(f.Geometry.Coordinates) >= 2 {
			a.Lon, a.Lat = f.Geometry.Coordinates[0], f.Geometry.Coordinates[1]
		}
		if a.CodeInsee == "" {
			continue // sans code commune, on ne sait pas quoi télécharger
		}
		out = append(out, a)
	}
	return out, nil
}

// ChercherAdresse interroge la Base Adresse Nationale.
func ChercherAdresse(requete string, limite int) ([]Adresse, error) {
	requete = strings.TrimSpace(requete)
	if requete == "" {
		return nil, nil
	}
	u := fmt.Sprintf("%s?q=%s&limit=%d&autocomplete=0",
		apiAdresse, url.QueryEscape(requete), limite)
	brut, err := getOctets(u)
	if err != nil {
		return nil, fmt.Errorf("recherche d'adresse : %w", err)
	}
	return parserAdresses(brut)
}

// ---------------------------------------------------------------------------
// Rapprochement des libellés de voie
// ---------------------------------------------------------------------------
//
// DVF écrit « AV DU PORT », la BAN « Avenue du Port ». Comparer les deux
// demande de ramener les deux écritures à une forme unique : majuscules sans
// accents, type de voie développé, particules retirées.

var typesVoie = map[string]string{
	"AV": "AVENUE", "AVE": "AVENUE", "BD": "BOULEVARD", "BLD": "BOULEVARD",
	"CHE": "CHEMIN", "CHEM": "CHEMIN", "IMP": "IMPASSE", "ALL": "ALLEE",
	"PL": "PLACE", "RTE": "ROUTE", "SQ": "SQUARE", "RES": "RESIDENCE",
	"LOT": "LOTISSEMENT", "RLE": "RUELLE", "SEN": "SENTE", "SENT": "SENTE",
	"PAS": "PASSAGE", "PASS": "PASSAGE", "CRS": "COURS", "FG": "FAUBOURG",
	"QU": "QUAI", "MTE": "MONTEE", "TRA": "TRAVERSE", "VLA": "VILLA",
	"HAM": "HAMEAU", "ESP": "ESPLANADE", "PROM": "PROMENADE", "RPT": "ROND POINT",
	"CHS": "CHAUSSEE", "DOM": "DOMAINE", "PLN": "PLAINE", "PTE": "PORTE",
	"GR": "GRANDE RUE", "VOI": "VOIE", "CAR": "CARREFOUR", "PARC": "PARC",
}

// particules : mots vides qui ne discriminent rien et que les deux sources
// n'écrivent pas toujours.
var particules = map[string]bool{
	"DE": true, "DU": true, "DES": true, "LA": true, "LE": true, "LES": true,
	"D": true, "L": true, "AU": true, "AUX": true, "ET": true, "A": true,
}

var accents = strings.NewReplacer(
	"À", "A", "Â", "A", "Ä", "A", "Á", "A", "Ã", "A", "Å", "A",
	"É", "E", "È", "E", "Ê", "E", "Ë", "E",
	"Î", "I", "Ï", "I", "Í", "I",
	"Ô", "O", "Ö", "O", "Ó", "O", "Õ", "O",
	"Ù", "U", "Û", "U", "Ü", "U", "Ú", "U",
	"Ç", "C", "Ñ", "N", "Œ", "OE", "Æ", "AE",
)

// cleVoie ramène un nom de voie à sa forme comparable.
func cleVoie(nom string) string {
	nom = accents.Replace(strings.ToUpper(strings.TrimSpace(nom)))
	var mots []string
	for _, mot := range strings.FieldsFunc(nom, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9')
	}) {
		if long, ok := typesVoie[mot]; ok {
			mot = long
		}
		if particules[mot] {
			continue
		}
		mots = append(mots, mot)
	}
	return strings.Join(mots, " ")
}

// memeVoie rapproche deux libellés de voie. L'inclusion rattrape les mentions
// que l'une des deux sources ajoute ou omet (« PROLONGEE », « ANCIENNE »).
func memeVoie(a, b string) bool {
	ca, cb := cleVoie(a), cleVoie(b)
	if ca == "" || cb == "" {
		return false
	}
	if ca == cb {
		return true
	}
	if len(ca) >= 6 && len(cb) >= 6 {
		return strings.Contains(ca, cb) || strings.Contains(cb, ca)
	}
	return false
}

// memeNumero compare deux numéros dans la voie en ignorant ce qui n'est pas
// chiffre : « 12 » et « 12 BIS » portent le même numéro.
func memeNumero(a, b string) bool {
	na, oka := numero(a)
	nb, okb := numero(b)
	return oka && okb && na == nb
}

func numero(v string) (int, bool) {
	var chiffres strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			chiffres.WriteRune(r)
			continue
		}
		if chiffres.Len() > 0 {
			break
		}
	}
	if chiffres.Len() == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(chiffres.String())
	return n, err == nil
}

// ---------------------------------------------------------------------------
// Page d'une adresse
// ---------------------------------------------------------------------------

// ventesVoie renvoie les ventes de la voie recherchée, tous millésimes et tous
// types confondus. Si la voie n'est reconnue nulle part, on ne renvoie pas une
// liste vide : la commune entière reste consultable, avec un avertissement.
func (m model) ventesVoie() ([]Sale, bool) {
	if m.adresse == nil {
		return nil, false
	}
	var out []Sale
	for _, s := range m.total.Sales {
		if memeVoie(s.Voie, m.adresse.Voie) {
			out = append(out, s)
		}
	}
	return out, len(out) > 0
}

// ventesAdresse alimente le tableau de l'onglet Adresse.
func (m model) ventesAdresse() []Sale {
	ventes, ok := m.ventesVoie()
	if !ok {
		ventes = m.total.Sales
	}
	return VentesTriees(ventes, "", m.tri)
}

// auNumero filtre les ventes portant exactement le numéro cherché.
func (m model) auNumero(ventes []Sale) []Sale {
	if m.adresse == nil || m.adresse.Numero == "" {
		return nil
	}
	var out []Sale
	for _, s := range ventes {
		if memeNumero(s.Numero, m.adresse.Numero) {
			out = append(out, s)
		}
	}
	return out
}

// premiereVenteAuNumero renvoie l'indice, dans le tableau, de la première vente
// au numéro cherché : c'est là que le curseur doit se poser en arrivant.
func (m model) premiereVenteAuNumero() int {
	for i, s := range m.ventesAdresse() {
		if m.adresse != nil && memeNumero(s.Numero, m.adresse.Numero) {
			return i
		}
	}
	return 0
}

func (m *model) vueAdresse() string {
	var b strings.Builder
	if m.adresse == nil {
		return "\n  " + styleWarn.Render("Aucune adresse recherchée.") + "\n"
	}
	a := *m.adresse

	b.WriteString("\n  " + styleSection.Render("ADRESSE RECHERCHÉE") + "\n\n")
	champ(&b, "Adresse", styleOK.Render(a.Label))
	champ(&b, "Commune", strings.TrimSpace(a.CodePostal+" "+a.Commune))
	champ(&b, "Code INSEE", a.CodeInsee)
	champ(&b, "Département", a.Departement())
	if a.Lat != 0 || a.Lon != 0 {
		champ(&b, "Coordonnées", fmt.Sprintf("%.5f, %.5f", a.Lat, a.Lon))
	}
	champ(&b, "Millésimes agrégés", fmt.Sprintf("%s  (%d ventes exploitables)",
		m.total.Annee, len(m.total.Sales)))

	ventesVoie, voieTrouvee := m.ventesVoie()

	// --- Ce qui s'est vendu à ce numéro -----------------------------------
	b.WriteString("\n  " + styleSection.Render("À CETTE ADRESSE") + "\n\n")
	auNumero := m.auNumero(ventesVoie)
	switch {
	case !a.EstPrecise():
		b.WriteString("  " + styleDim.Render(
			"La recherche ne porte pas sur un numéro précis : voir la voie et la commune ci-dessous.") + "\n")
	case len(auNumero) == 0:
		b.WriteString("  " + styleDim.Render(fmt.Sprintf(
			"Aucune vente au %s sur les millésimes disponibles (%s).", a.Numero, m.total.Annee)) + "\n")
		b.WriteString("  " + styleDim.Render(
			"C'est le cas le plus fréquent : un logement change de mains tous les dix ans environ.") + "\n")
	default:
		for _, s := range auNumero {
			b.WriteString(fmt.Sprintf("  %s  %s  %-13s %8s %10s  %s\n",
				s.Date.Format("02/01/2006"), styleOK.Render(fmt.Sprintf("%13s", euros(s.Valeur))),
				typeCourt(s.Type), m2(s.SurfaceRef()), eurosM2(s.PrixM2Ref()),
				styleDim.Render(s.Adresse())))
		}
		b.WriteString("\n  " + styleDim.Render(
			"↵ sur la ligne correspondante du tableau ci-dessous pour la fiche complète.") + "\n")
	}

	// --- La voie ----------------------------------------------------------
	b.WriteString("\n  " + styleSection.Render("LA VOIE — "+strings.ToUpper(a.Voie)) + "\n\n")
	if !voieTrouvee {
		b.WriteString("  " + styleWarn.Render(
			"Cette voie n'apparaît dans aucune vente : le tableau ci-dessous porte sur toute la commune.") + "\n")
	} else {
		b.WriteString(enteteTypes + "\n")
		for _, r := range ResumerParType(ventesVoie) {
			ecart, ok := EcartRelatif(r.PrixM2Med, ResumerType(m.total.Sales, r.Type).PrixM2Med)
			b.WriteString(ligneType(r) + "   " +
				styleDim.Render(pourcent(ecart, ok)+" vs commune") + "\n")
		}
	}

	// --- La commune -------------------------------------------------------
	b.WriteString("\n  " + styleSection.Render("LA COMMUNE — TOUS MILLÉSIMES") + "\n\n")
	b.WriteString(enteteTypes + "\n")
	for _, r := range ResumerParType(m.total.Sales) {
		b.WriteString(ligneType(r) + "\n")
	}
	b.WriteString("\n  " + styleDim.Render(
		"L'onglet Résumé détaille l'évolution millésime par millésime.") + "\n")

	m.ecrireTableau(&b, m.ventesAdresse())
	return b.String()
}

// enteteTypes et ligneType : le tableau de synthèse par type de bien, partagé
// par la page d'adresse et le résumé de la commune.
var enteteTypes = styleDim.Render(
	"  Type            Ventes    Prix médian     Médiane €/m²   Surface méd.")

func ligneType(r ResumeType) string {
	return fmt.Sprintf("  %-14s %6d  %13s  %15s  %13s",
		typeCourt(r.Type), r.Nb, euros(r.PrixMed), eurosM2(r.PrixM2Med), m2(r.SurfMed))
}

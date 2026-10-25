package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Modèle de données
// ---------------------------------------------------------------------------

// Lot est un lot de copropriété rattaché à une ligne (numéro + surface Carrez).
type Lot struct {
	Numero string
	Carrez float64
}

// Row est une ligne brute du CSV DVF (une mutation x une parcelle x un local).
// Toutes les colonnes du fichier y sont conservées : la fiche détaillée d'une
// vente les restitue, et ce qui n'est pas lu ici est définitivement perdu pour
// l'affichage.
type Row struct {
	IDMutation        string
	Date              time.Time
	NumeroDisposition string
	Nature            string
	Valeur            float64

	Numero   string // numéro dans la voie
	Suffixe  string // bis, ter…
	Voie     string
	CodeVoie string

	CodePostal        string
	CodeCommune       string
	Commune           string
	CodeDepartement   string
	AncienCodeCommune string
	AncienNomCommune  string

	IDParcelle       string
	AncienIDParcelle string
	NumeroVolume     string

	Lots       []Lot
	NombreLots int

	CodeTypeLocal string
	TypeLocal     string
	SurfaceBati   float64
	Pieces        int

	CodeNatureCulture         string
	NatureCulture             string
	CodeNatureCultureSpeciale string
	NatureCultureSpeciale     string
	SurfaceTerrain            float64

	Longitude float64
	Latitude  float64
}

// Sale est une vente consolidée : une mutation = un bien exploitable.
// Lignes conserve les lignes brutes dont elle est issue : les statistiques
// n'en ont pas besoin, la fiche détaillée si.
type Sale struct {
	ID             string
	Date           time.Time
	Valeur         float64
	Numero         string
	Suffixe        string
	Voie           string
	CodePostal     string
	Commune        string
	Type           string // Maison, Appartement, Local commercial, Terrain
	SurfaceBati    float64
	Pieces         int
	SurfaceTerrain float64
	Lignes         []Row
}

// Adresse renvoie « 12 bis RUE DES LILAS », sans la commune.
func (s Sale) Adresse() string {
	morceaux := []string{}
	for _, v := range []string{s.Numero, s.Suffixe, s.Voie} {
		if !champVide(v) {
			morceaux = append(morceaux, strings.TrimSpace(v))
		}
	}
	if len(morceaux) == 0 {
		return "adresse non renseignée"
	}
	return strings.Join(morceaux, " ")
}

// Ville renvoie « 10000 TROYES » à partir des lignes de la mutation.
func (s Sale) Ville() string {
	cp, nom := strings.TrimSpace(s.CodePostal), strings.TrimSpace(s.Commune)
	switch {
	case cp != "" && nom != "":
		return cp + " " + nom
	case nom != "":
		return nom
	case cp != "":
		return cp
	}
	return ""
}

// Position renvoie les coordonnées de la première ligne géolocalisée.
func (s Sale) Position() (float64, float64, bool) {
	for _, r := range s.Lignes {
		if r.Longitude != 0 || r.Latitude != 0 {
			return r.Latitude, r.Longitude, true
		}
	}
	return 0, 0, false
}

// SubTerrain est une subdivision de parcelle : c'est l'unité réellement
// mesurée par le fichier (une parcelle est découpée par nature de culture).
type SubTerrain struct {
	Parcelle string
	Culture  string
	Surface  float64
}

// Parcelles renvoie les subdivisions de terrain de la mutation, dédoublonnées
// exactement comme le fait la consolidation : la somme de leurs surfaces vaut
// SurfaceTerrain.
func (s Sale) Parcelles() []SubTerrain {
	var out []SubTerrain
	vues := map[string]bool{}
	for _, r := range s.Lignes {
		if r.IDParcelle == "" || r.SurfaceTerrain <= 0 {
			continue
		}
		cle := cleSubTerrain(r)
		if vues[cle] {
			continue
		}
		vues[cle] = true
		culture := r.NatureCulture
		if culture == "" {
			culture = r.CodeNatureCulture
		}
		if r.NatureCultureSpeciale != "" {
			culture += " (" + r.NatureCultureSpeciale + ")"
		}
		out = append(out, SubTerrain{Parcelle: r.IDParcelle, Culture: culture, Surface: r.SurfaceTerrain})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Parcelle != out[j].Parcelle {
			return out[i].Parcelle < out[j].Parcelle
		}
		return out[i].Surface > out[j].Surface
	})
	return out
}

// Locaux renvoie les locaux distincts de la mutation (type + surface + pièces),
// dédoublonnés : la même ligne est répétée pour chaque parcelle.
func (s Sale) Locaux() []Row {
	var out []Row
	vues := map[string]bool{}
	for _, r := range s.Lignes {
		if champVide(r.TypeLocal) {
			continue
		}
		cle := fmt.Sprintf("%s|%.0f|%d", r.TypeLocal, r.SurfaceBati, r.Pieces)
		if vues[cle] {
			continue
		}
		vues[cle] = true
		out = append(out, r)
	}
	return out
}

// LotsCopro renvoie les lots de copropriété distincts de la mutation.
func (s Sale) LotsCopro() []Lot {
	var out []Lot
	vus := map[string]bool{}
	for _, r := range s.Lignes {
		for _, l := range r.Lots {
			if vus[l.Numero] {
				continue
			}
			vus[l.Numero] = true
			out = append(out, l)
		}
	}
	return out
}

// PrixM2 renvoie le prix au m² bâti (0 si pas de bâti).
func (s Sale) PrixM2() float64 {
	if s.SurfaceBati <= 0 {
		return 0
	}
	return s.Valeur / s.SurfaceBati
}

// PrixM2Terrain renvoie le prix au m² de terrain (pour les terrains nus).
func (s Sale) PrixM2Terrain() float64 {
	if s.SurfaceTerrain <= 0 {
		return 0
	}
	return s.Valeur / s.SurfaceTerrain
}

// Dataset regroupe les ventes d'une ville pour une année.
type Dataset struct {
	Ville     string
	Annee     string
	Fichiers  int
	Mutations int
	Sales     []Sale
	Ecartees  int // mutations écartées (ventes globales, échanges, prix nul)

	// Deux dates qu'on confond facilement : celle où l'État a publié le
	// fichier, et celle de la vente la plus récente qu'il contient — DVF
	// accuse plusieurs mois de retard sur les signatures. Zéro : inconnue.
	Publication   time.Time
	DerniereVente time.Time

	// Provisoire : au moins un millésime vient du fichier brut de la DGFiP,
	// sans coordonnées, en attendant geo-dvf.
	Provisoire bool
}

// Types de locaux considérés comme "bien principal".
var typesPrincipaux = map[string]bool{
	"Maison":      true,
	"Appartement": true,
	"Local industriel. commercial ou assimilé": true,
}

// ---------------------------------------------------------------------------
// Lecture des fichiers
// ---------------------------------------------------------------------------

func champVide(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || v == "None" || v == "nan" || v == "NaN"
}

func parseFloat(v string) float64 {
	if champVide(v) {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0
	}
	return f
}

func parseInt(v string) int {
	return int(parseFloat(v))
}

// LireDossier lit tous les CSV d'un dossier et renvoie les lignes brutes.
func LireDossier(dir string) ([]Row, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}

	var rows []Row
	nbFichiers := 0

	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".csv") {
			continue
		}
		fichierRows, err := lireCSV(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // fichier illisible : on l'ignore silencieusement
		}
		rows = append(rows, fichierRows...)
		nbFichiers++
	}
	return rows, nbFichiers, nil
}

func lireCSV(path string) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Le séparateur varie selon la provenance : les archives départementales
	// publiées sont en virgules, certains extraits diffusés par commune en
	// points-virgules. On le déduit de l'en-tête plutôt que de l'imposer.
	tete := make([]byte, 4096)
	n, _ := f.Read(tete)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	r := csv.NewReader(f)
	r.Comma = detecterSeparateur(string(tete[:n]))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil, err
	}

	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(strings.Trim(h, "\ufeff\r\n"))] = i
	}

	get := func(rec []string, col string) string {
		i, ok := idx[col]
		if !ok || i >= len(rec) {
			return ""
		}
		return rec[i]
	}

	var rows []Row
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		d, _ := time.Parse("2006-01-02", strings.TrimSpace(get(rec, "date_mutation")))
		txt := func(col string) string {
			v := strings.TrimSpace(get(rec, col))
			if champVide(v) {
				return ""
			}
			return v
		}

		// Les cinq lots possibles sont des colonnes numérotées : on les replie
		// en une tranche pour ne pas traîner dix champs presque toujours vides.
		var lots []Lot
		for i := 1; i <= 5; i++ {
			num := txt(fmt.Sprintf("lot%d_numero", i))
			carrez := parseFloat(get(rec, fmt.Sprintf("lot%d_surface_carrez", i)))
			if num == "" && carrez <= 0 {
				continue
			}
			lots = append(lots, Lot{Numero: num, Carrez: carrez})
		}

		rows = append(rows, Row{
			IDMutation:        txt("id_mutation"),
			Date:              d,
			NumeroDisposition: txt("numero_disposition"),
			Nature:            txt("nature_mutation"),
			Valeur:            parseFloat(get(rec, "valeur_fonciere")),

			Numero:   txt("adresse_numero"),
			Suffixe:  txt("adresse_suffixe"),
			Voie:     txt("adresse_nom_voie"),
			CodeVoie: txt("adresse_code_voie"),

			CodePostal:        txt("code_postal"),
			CodeCommune:       txt("code_commune"),
			Commune:           txt("nom_commune"),
			CodeDepartement:   txt("code_departement"),
			AncienCodeCommune: txt("ancien_code_commune"),
			AncienNomCommune:  txt("ancien_nom_commune"),

			IDParcelle:       txt("id_parcelle"),
			AncienIDParcelle: txt("ancien_id_parcelle"),
			NumeroVolume:     txt("numero_volume"),

			Lots:       lots,
			NombreLots: parseInt(get(rec, "nombre_lots")),

			CodeTypeLocal: txt("code_type_local"),
			TypeLocal:     txt("type_local"),
			SurfaceBati:   parseFloat(get(rec, "surface_reelle_bati")),
			Pieces:        parseInt(get(rec, "nombre_pieces_principales")),

			CodeNatureCulture:         txt("code_nature_culture"),
			NatureCulture:             txt("nature_culture"),
			CodeNatureCultureSpeciale: txt("code_nature_culture_speciale"),
			NatureCultureSpeciale:     txt("nature_culture_speciale"),
			SurfaceTerrain:            parseFloat(get(rec, "surface_terrain")),

			Longitude: parseFloat(get(rec, "longitude")),
			Latitude:  parseFloat(get(rec, "latitude")),
		})
	}
	return rows, nil
}

// ---------------------------------------------------------------------------
// Consolidation : lignes brutes -> ventes exploitables
// ---------------------------------------------------------------------------

// Consolider regroupe les lignes par mutation et écarte les cas non exploitables :
//   - nature de mutation autre que "Vente" (échange, adjudication...)
//   - valeur foncière absente
//   - vente globale portant sur plusieurs biens distincts (le prix ne peut pas
//     être rattaché à un seul bien)
//
// Un même bien réparti sur plusieurs parcelles cadastrales n'est PAS considéré
// comme une anomalie : c'est un cas fréquent et parfaitement normal.
// cleSubTerrain identifie une subdivision de parcelle. La même subdivision est
// répétée pour chaque local de la mutation : sans cette clé, on la compterait
// autant de fois.
func cleSubTerrain(r Row) string {
	return fmt.Sprintf("%s|%s|%.0f", r.IDParcelle, r.CodeNatureCulture, r.SurfaceTerrain)
}

// nouvelleVente assemble une vente à partir de la ligne de référence et de
// toutes les lignes de la mutation, conservées pour la fiche détaillée.
func nouvelleVente(id string, ref Row, lignes []Row, typeBien string, surface float64, pieces int, terrain float64) Sale {
	return Sale{
		ID: id, Date: ref.Date, Valeur: ref.Valeur,
		Numero: ref.Numero, Suffixe: ref.Suffixe, Voie: ref.Voie,
		CodePostal: ref.CodePostal, Commune: ref.Commune,
		Type: typeBien, SurfaceBati: surface, Pieces: pieces,
		SurfaceTerrain: terrain,
		Lignes:         append([]Row(nil), lignes...),
	}
}

func Consolider(rows []Row) ([]Sale, int, int) {
	groupes := map[string][]Row{}
	ordre := []string{}
	for _, r := range rows {
		if _, vu := groupes[r.IDMutation]; !vu {
			ordre = append(ordre, r.IDMutation)
		}
		groupes[r.IDMutation] = append(groupes[r.IDMutation], r)
	}

	var sales []Sale
	ecartees := 0

	for _, id := range ordre {
		g := groupes[id]
		ref := g[0]

		if ref.Nature != "Vente" || ref.Valeur <= 0 {
			ecartees++
			continue
		}

		// Surfaces distinctes par type de bien principal.
		surfacesParType := map[string]map[float64]bool{}
		for _, r := range g {
			if !typesPrincipaux[r.TypeLocal] {
				continue
			}
			if surfacesParType[r.TypeLocal] == nil {
				surfacesParType[r.TypeLocal] = map[float64]bool{}
			}
			surfacesParType[r.TypeLocal][r.SurfaceBati] = true
		}

		// Plusieurs types principaux différents => vente globale, on écarte.
		if len(surfacesParType) > 1 {
			ecartees++
			continue
		}

		// Terrain : une parcelle est découpée en lignes selon la nature de
		// culture (sols, terrains d'agrément, taillis...), et l'ensemble est
		// répété pour chaque local de la mutation. On somme donc les
		// subdivisions distinctes, identifiées par parcelle + culture + surface.
		vues := map[string]bool{}
		terrain := 0.0
		for _, r := range g {
			if r.IDParcelle == "" || r.SurfaceTerrain <= 0 {
				continue
			}
			cle := cleSubTerrain(r)
			if vues[cle] {
				continue
			}
			vues[cle] = true
			terrain += r.SurfaceTerrain
		}

		// Aucun bien bâti : c'est un terrain nu.
		if len(surfacesParType) == 0 {
			if terrain <= 0 {
				ecartees++
				continue
			}
			sales = append(sales, nouvelleVente(id, ref, g, "Terrain", 0, 0, terrain))
			continue
		}

		// Un seul type principal.
		var typeBien string
		var surfaces map[float64]bool
		for t, s := range surfacesParType {
			typeBien, surfaces = t, s
		}

		// Plusieurs surfaces différentes pour ce type => plusieurs biens vendus
		// ensemble (ex : 2 maisons), le prix unitaire est inconnu.
		if len(surfaces) > 1 {
			ecartees++
			continue
		}

		var surface float64
		for s := range surfaces {
			surface = s
		}
		if surface <= 0 {
			ecartees++
			continue
		}

		pieces := 0
		for _, r := range g {
			if r.TypeLocal == typeBien && r.Pieces > pieces {
				pieces = r.Pieces
			}
		}

		sales = append(sales, nouvelleVente(id, ref, g, typeBien, surface, pieces, terrain))
	}

	return sales, len(ordre), ecartees
}

// ChargerDataset lit un dossier année et renvoie le jeu de données consolidé.
func ChargerDataset(ville, annee, dir string) (Dataset, error) {
	rows, nbFichiers, err := LireDossier(dir)
	if err != nil {
		return Dataset{}, err
	}
	sales, mutations, ecartees := Consolider(rows)
	return Dataset{
		Ville: ville, Annee: annee, Fichiers: nbFichiers,
		Mutations: mutations, Sales: sales, Ecartees: ecartees,
		Publication:   LirePublication(filepath.Join(dir, fichierPublication)),
		DerniereVente: derniereVente(sales),
		Provisoire:    fichierExiste(filepath.Join(dir, fichierProvisoire)),
	}, nil
}

func derniereVente(sales []Sale) time.Time {
	var t time.Time
	for _, s := range sales {
		if s.Date.After(t) {
			t = s.Date
		}
	}
	return t
}

// ---------------------------------------------------------------------------
// Statistiques
// ---------------------------------------------------------------------------

func mediane(v []float64) float64 { return quantile(v, 0.5) }

// quantile renvoie le quantile q (0..1) d'une série non triée.
func quantile(v []float64, q float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s) == 1 {
		return s[0]
	}
	pos := q * float64(len(s)-1)
	bas := int(pos)
	haut := bas + 1
	if haut >= len(s) {
		return s[len(s)-1]
	}
	frac := pos - float64(bas)
	return s[bas]*(1-frac) + s[haut]*frac
}

func moyenne(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	total := 0.0
	for _, x := range v {
		total += x
	}
	return total / float64(len(v))
}

func minMax(v []float64) (float64, float64) {
	if len(v) == 0 {
		return 0, 0
	}
	mn, mx := v[0], v[0]
	for _, x := range v {
		if x < mn {
			mn = x
		}
		if x > mx {
			mx = x
		}
	}
	return mn, mx
}

// StatsType regroupe les indicateurs utiles à un acheteur pour un type de bien.
type StatsType struct {
	Type    string
	Nb      int
	Prix    Serie // valeurs foncières
	PrixM2  Serie // prix au m² bâti
	Surface Serie // surface bâtie
	Terrain Serie // surface de terrain
}

// Serie porte les indicateurs de dispersion d'une série de valeurs.
type Serie struct {
	Nb                    int
	Min, Q1, Med, Q3, Max float64
	Moy                   float64
}

func nouvelleSerie(v []float64) Serie {
	mn, mx := minMax(v)
	return Serie{
		Nb: len(v), Min: mn, Max: mx,
		Q1: quantile(v, 0.25), Med: mediane(v), Q3: quantile(v, 0.75),
		Moy: moyenne(v),
	}
}

// StatsPourType calcule les statistiques d'un type de bien donné.
func StatsPourType(sales []Sale, typeBien string) StatsType {
	var prix, prixM2, surf, terr []float64
	for _, s := range sales {
		if s.Type != typeBien {
			continue
		}
		prix = append(prix, s.Valeur)
		if p := s.PrixM2(); p > 0 {
			prixM2 = append(prixM2, p)
		}
		if s.SurfaceBati > 0 {
			surf = append(surf, s.SurfaceBati)
		}
		if s.SurfaceTerrain > 0 {
			terr = append(terr, s.SurfaceTerrain)
		}
	}
	return StatsType{
		Type: typeBien, Nb: len(prix),
		Prix: nouvelleSerie(prix), PrixM2: nouvelleSerie(prixM2),
		Surface: nouvelleSerie(surf), Terrain: nouvelleSerie(terr),
	}
}

// Tranche est une ligne du tableau "par tranche de surface".
type Tranche struct {
	Label     string
	Nb        int
	PrixMed   float64
	PrixM2Med float64
}

var bornesSurface = []struct {
	Label    string
	Min, Max float64
}{
	{"< 50 m²", 0, 50},
	{"50 – 70 m²", 50, 70},
	{"70 – 90 m²", 70, 90},
	{"90 – 110 m²", 90, 110},
	{"110 – 130 m²", 110, 130},
	{"130 – 150 m²", 130, 150},
	{"> 150 m²", 150, 1e9},
}

// ParTrancheSurface répartit les ventes par tranche de surface bâtie.
// C'est le tableau le plus directement utile pour caler un budget d'achat.
func ParTrancheSurface(sales []Sale, typeBien string) []Tranche {
	var out []Tranche
	for _, b := range bornesSurface {
		var prix, pm2 []float64
		for _, s := range sales {
			if s.Type != typeBien || s.SurfaceBati <= 0 {
				continue
			}
			if s.SurfaceBati >= b.Min && s.SurfaceBati < b.Max {
				prix = append(prix, s.Valeur)
				pm2 = append(pm2, s.PrixM2())
			}
		}
		if len(prix) == 0 {
			continue
		}
		out = append(out, Tranche{
			Label: b.Label, Nb: len(prix),
			PrixMed: mediane(prix), PrixM2Med: mediane(pm2),
		})
	}
	return out
}

// ParPieces répartit les ventes par nombre de pièces principales.
func ParPieces(sales []Sale, typeBien string) []Tranche {
	buckets := map[int][]float64{}
	m2 := map[int][]float64{}
	for _, s := range sales {
		if s.Type != typeBien || s.Pieces <= 0 {
			continue
		}
		p := s.Pieces
		if p > 6 {
			p = 6
		}
		buckets[p] = append(buckets[p], s.Valeur)
		if v := s.PrixM2(); v > 0 {
			m2[p] = append(m2[p], v)
		}
	}
	var cles []int
	for k := range buckets {
		cles = append(cles, k)
	}
	sort.Ints(cles)

	var out []Tranche
	for _, k := range cles {
		label := strconv.Itoa(k) + " pièces"
		if k == 1 {
			label = "1 pièce"
		}
		if k == 6 {
			label = "6 pièces et +"
		}
		out = append(out, Tranche{
			Label: label, Nb: len(buckets[k]),
			PrixMed: mediane(buckets[k]), PrixM2Med: mediane(m2[k]),
		})
	}
	return out
}

// VentesParMois renvoie le nombre de ventes pour chacun des 12 mois.
func VentesParMois(sales []Sale) [12]int {
	var out [12]int
	for _, s := range sales {
		if s.Date.IsZero() {
			continue
		}
		out[int(s.Date.Month())-1]++
	}
	return out
}

// VoieActive décrit l'activité sur une voie donnée.
type VoieActive struct {
	Nom     string
	Nb      int
	PrixMed float64
}

// TopVoies renvoie les voies les plus actives : utile pour repérer les
// micro-marchés (lotissements récents, quartiers qui tournent).
func TopVoies(sales []Sale, n int) []VoieActive {
	buckets := map[string][]float64{}
	for _, s := range sales {
		if s.Voie == "" || champVide(s.Voie) {
			continue
		}
		buckets[s.Voie] = append(buckets[s.Voie], s.Valeur)
	}
	var out []VoieActive
	for nom, prix := range buckets {
		out = append(out, VoieActive{Nom: nom, Nb: len(prix), PrixMed: mediane(prix)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Nb != out[j].Nb {
			return out[i].Nb > out[j].Nb
		}
		return out[i].Nom < out[j].Nom
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// StatsTerrain calcule le prix au m² des terrains nus (utile pour construire).
func StatsTerrain(sales []Sale) (Serie, Serie) {
	var prix, pm2 []float64
	for _, s := range sales {
		if s.Type != "Terrain" {
			continue
		}
		prix = append(prix, s.Valeur)
		if v := s.PrixM2Terrain(); v > 0 {
			pm2 = append(pm2, v)
		}
	}
	return nouvelleSerie(prix), nouvelleSerie(pm2)
}

// ---------------------------------------------------------------------------
// Liste des ventes
// ---------------------------------------------------------------------------

// Tri désigne le critère de classement du tableau des ventes.
type Tri int

const (
	TriDate Tri = iota
	TriPrix
	TriPrixM2
	TriSurface
)

// LibellesTri décrit chaque critère, dans l'ordre des constantes.
var LibellesTri = []string{"date ↓", "prix ↓", "€/m² ↓", "surface ↓"}

// SurfaceRef est la surface qui caractérise le bien : la surface bâtie, ou
// celle du terrain pour un terrain nu.
func (s Sale) SurfaceRef() float64 {
	if s.Type == "Terrain" {
		return s.SurfaceTerrain
	}
	return s.SurfaceBati
}

// PrixM2Ref est le prix au m² pertinent pour le type de bien.
func (s Sale) PrixM2Ref() float64 {
	if s.Type == "Terrain" {
		return s.PrixM2Terrain()
	}
	return s.PrixM2()
}

// VentesTriees renvoie les ventes d'un type donné (tous types si typeBien est
// vide), classées selon le critère.
// Le tri est toujours total (départage par date puis par identifiant) pour que
// l'ordre soit stable d'un affichage à l'autre.
func VentesTriees(sales []Sale, typeBien string, tri Tri) []Sale {
	var out []Sale
	for _, s := range sales {
		if typeBien == "" || s.Type == typeBien {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch tri {
		case TriPrix:
			if a.Valeur != b.Valeur {
				return a.Valeur > b.Valeur
			}
		case TriPrixM2:
			if a.PrixM2Ref() != b.PrixM2Ref() {
				return a.PrixM2Ref() > b.PrixM2Ref()
			}
		case TriSurface:
			if a.SurfaceRef() != b.SurfaceRef() {
				return a.SurfaceRef() > b.SurfaceRef()
			}
		}
		if !a.Date.Equal(b.Date) {
			return a.Date.After(b.Date)
		}
		return a.ID < b.ID
	})
	return out
}

// EcartRelatif renvoie l'écart de v à la référence, en pourcentage
// (+12 = 12 % au-dessus). Le booléen dit si le calcul a un sens.
func EcartRelatif(v, reference float64) (float64, bool) {
	if v <= 0 || reference <= 0 {
		return 0, false
	}
	return (v/reference - 1) * 100, true
}

// ---------------------------------------------------------------------------
// Agrégation de plusieurs millésimes
// ---------------------------------------------------------------------------

// Fusionner agrège plusieurs millésimes en un seul jeu de données. Les
// identifiants de mutation portent l'année, les ventes ne peuvent donc pas se
// dédoublonner d'un millésime à l'autre : une simple concaténation suffit.
//
// L'intérêt n'est pas cosmétique : sur une commune moyenne, une année compte
// quelques dizaines de ventes par type de bien — trop peu pour qu'une médiane
// par tranche de surface veuille dire quoi que ce soit. Six millésimes cumulés
// donnent enfin des effectifs exploitables, au prix d'une hypothèse à garder en
// tête : le marché est supposé comparable sur toute la période.
func Fusionner(jeux []Dataset) Dataset {
	if len(jeux) == 0 {
		return Dataset{}
	}
	out := Dataset{Ville: jeux[0].Ville}
	var annees []string
	out.Publication = jeux[0].Publication
	for _, d := range jeux {
		// La publication retenue est la plus ancienne : c'est elle qui borne
		// la fraîcheur de l'ensemble. Une seule inconnue rend le tout inconnu.
		if d.Publication.IsZero() || d.Publication.Before(out.Publication) {
			out.Publication = d.Publication
		}
		out.Provisoire = out.Provisoire || d.Provisoire
		if d.DerniereVente.After(out.DerniereVente) {
			out.DerniereVente = d.DerniereVente
		}
		out.Fichiers += d.Fichiers
		out.Mutations += d.Mutations
		out.Ecartees += d.Ecartees
		out.Sales = append(out.Sales, d.Sales...)
		if d.Annee != "" {
			annees = append(annees, d.Annee)
		}
	}
	sort.Strings(annees)
	switch len(annees) {
	case 0:
	case 1:
		out.Annee = annees[0]
	default:
		out.Annee = annees[0] + " – " + annees[len(annees)-1]
	}
	return out
}

// LigneAnnee est une ligne du tableau d'évolution : un millésime, pour un type
// de bien donné.
type LigneAnnee struct {
	Annee     string
	Nb        int
	PrixMed   float64
	PrixM2Med float64
	SurfMed   float64
}

// EvolutionParAnnee suit un type de bien millésime après millésime. C'est la
// seule statistique de ce programme qui réponde à « le marché monte-t-il ? » —
// aucun indicateur calculé sur une seule année ne peut le dire.
//
// typeBien vide agrège tous les types.
func EvolutionParAnnee(jeux []Dataset, typeBien string) []LigneAnnee {
	var out []LigneAnnee
	for _, d := range jeux {
		var prix, pm2, surf []float64
		for _, s := range d.Sales {
			if typeBien != "" && s.Type != typeBien {
				continue
			}
			prix = append(prix, s.Valeur)
			if v := s.PrixM2Ref(); v > 0 {
				pm2 = append(pm2, v)
			}
			if v := s.SurfaceRef(); v > 0 {
				surf = append(surf, v)
			}
		}
		if len(prix) == 0 {
			continue
		}
		out = append(out, LigneAnnee{
			Annee: d.Annee, Nb: len(prix),
			PrixMed: mediane(prix), PrixM2Med: mediane(pm2), SurfMed: mediane(surf),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Annee < out[j].Annee })
	return out
}

// ResumeType est la ligne de synthèse d'un type de bien : ce qu'on affiche
// quand on compare plusieurs types côte à côte.
//
// À la différence de StatsPourType, les indicateurs sont pris sur la surface et
// le prix au m² *pertinents pour le type* : un terrain nu n'a pas de surface
// bâtie, et l'afficher vide au lieu de sa surface de terrain revient à ne rien
// dire de la moitié du marché.
type ResumeType struct {
	Type      string
	Nb        int
	PrixMed   float64
	PrixM2Med float64
	SurfMed   float64
}

// ResumerType synthétise un type de bien.
func ResumerType(sales []Sale, typeBien string) ResumeType {
	var prix, pm2, surf []float64
	for _, s := range sales {
		if s.Type != typeBien {
			continue
		}
		prix = append(prix, s.Valeur)
		if v := s.PrixM2Ref(); v > 0 {
			pm2 = append(pm2, v)
		}
		if v := s.SurfaceRef(); v > 0 {
			surf = append(surf, v)
		}
	}
	return ResumeType{
		Type: typeBien, Nb: len(prix),
		PrixMed: mediane(prix), PrixM2Med: mediane(pm2), SurfMed: mediane(surf),
	}
}

// ResumerParType synthétise tous les types présents, les plus courants d'abord.
func ResumerParType(sales []Sale) []ResumeType {
	vus := map[string]bool{}
	var out []ResumeType
	for _, s := range sales {
		if vus[s.Type] {
			continue
		}
		vus[s.Type] = true
		out = append(out, ResumerType(sales, s.Type))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Nb != out[j].Nb {
			return out[i].Nb > out[j].Nb
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func fichierExiste(chemin string) bool {
	_, err := os.Stat(chemin)
	return err == nil
}

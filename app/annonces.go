package main

// ---------------------------------------------------------------------------
// Les biens actuellement en vente
// ---------------------------------------------------------------------------
//
// DVF dit ce qui s'est vendu ; ce fichier dit ce qui est à vendre. Les deux
// n'ont ni la même fraîcheur ni la même garantie : DVF est un acte notarié,
// une annonce est un prix demandé par un vendeur, révisable et parfois jamais
// atteint. On ne les mélange donc pas dans un même tableau — mais les comparer
// sur une commune est tout l'intérêt.
//
// Aucune source n'est privilégiée : un portail est décrit par l'interface
// SourceAnnonces et s'enregistre au démarrage. Le cœur du programme ne connaît
// que Criteres et Annonce.

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Le bien
// ---------------------------------------------------------------------------

// TypeBien classe les annonces par nature de bien. Chaque portail a sa propre
// nomenclature — bien plus fine, et différente d'un portail à l'autre : la
// traduction se fait dans l'adaptateur, pas ici.
type TypeBien int

const (
	BienInconnu TypeBien = iota
	BienMaison
	BienAppartement
	BienTerrain
)

// LibellesTypeBien : l'ordre suit celui de la constante.
var LibellesTypeBien = []string{"Autre", "Maison", "Appartement", "Terrain nu"}

func (t TypeBien) String() string {
	if int(t) < len(LibellesTypeBien) {
		return LibellesTypeBien[t]
	}
	return LibellesTypeBien[BienInconnu]
}

// TousTypes est la sélection par défaut : les trois natures de bien qui nous
// intéressent, à l'exclusion des parkings, caves et locaux commerciaux.
func TousTypes() []TypeBien { return []TypeBien{BienMaison, BienAppartement, BienTerrain} }

// Annonce est un bien en vente, normalisé. Les champs numériques valent zéro
// quand le portail ne les publie pas : un terrain n'a pas de surface habitable,
// un appartement n'a pas de terrain, et beaucoup d'annonces taisent le nombre
// de pièces. Zéro se lit donc « non renseigné », jamais « nul » — les filtres
// en tiennent compte explicitement.
type Annonce struct {
	Source string // nom du portail d'origine
	ID     string
	URL    string
	Titre  string

	Type           TypeBien
	Prix           float64 // en euros
	Surface        float64 // habitable, m²
	SurfaceTerrain float64 // m²
	Pieces         int

	Commune     string
	CodePostal  string
	Departement string
	Lat, Lon    float64 // 0,0 quand la position est inconnue

	Neuf    bool
	Pro     bool // annonce déposée par un professionnel
	Publiee time.Time

	// PublieeAvant : quand le portail masque la date de publication, la seule
	// chose qu'on sait est qu'elle est antérieure à ce seuil. Bien'ici le fait
	// pour toute annonce de plus de deux mois. Exclusif de Publiee.
	PublieeAvant time.Time

	// PrixActe : le prix tel qu'il figurera dans l'acte de vente, donc dans
	// DVF. Il diffère du prix affiché quand les honoraires d'agence sont à la
	// charge de l'acheteur : l'annonce les inclut, l'acte non. Zéro : inconnu,
	// on retombe sur le prix affiché.
	PrixActe float64

	// Viager : le prix affiché n'est que le bouquet, la rente vient en plus.
	// Il ne se compare à aucune vente ordinaire.
	Viager bool
}

// LibellePublication dit quand l'annonce a été publiée, ou ce qu'on en sait.
// Court, pour tenir dans une colonne : « 12/10/26 », « <28/08/26 », « – ».
func (a Annonce) LibellePublication() string {
	switch {
	case !a.Publiee.IsZero():
		return a.Publiee.Format("02/01/06")
	case !a.PublieeAvant.IsZero():
		return "<" + a.PublieeAvant.Format("02/01/06")
	default:
		return "–"
	}
}

// PrixCompare est le prix à confronter à DVF.
func (a Annonce) PrixCompare() float64 {
	if a.PrixActe > 0 {
		return a.PrixActe
	}
	return a.Prix
}

// PrixM2 vaut zéro quand il n'a pas de sens — un terrain nu, ou une annonce
// sans surface. Retourner zéro plutôt qu'une erreur : l'affichage sait déjà
// écrire « — » pour une valeur absente, et aucun appelant ne pourrait faire
// mieux d'une erreur ici.
func (a Annonce) PrixM2() float64 {
	if a.Surface <= 0 || a.Prix <= 0 {
		return 0
	}
	return a.Prix / a.Surface
}

// Localisee indique si l'annonce porte une position exploitable. Les portails
// floutent la position exacte d'un bien — on obtient le centre d'un quartier ou
// d'une commune, ce qui suffit pour un rayon de plusieurs kilomètres mais
// interdit d'en tirer une adresse.
func (a Annonce) Localisee() bool { return a.Lat != 0 || a.Lon != 0 }

// ---------------------------------------------------------------------------
// Où chercher
// ---------------------------------------------------------------------------

// NiveauZone dit à quel échelon administratif on cherche.
type NiveauZone int

const (
	ZoneCommune NiveauZone = iota
	ZoneDepartement
	ZoneRegion
)

var libellesNiveauZone = []string{"commune", "département", "région"}

func (n NiveauZone) String() string {
	if int(n) < len(libellesNiveauZone) {
		return libellesNiveauZone[n]
	}
	return "zone"
}

// ZoneAnnonce est une zone de recherche résolue chez un portail donné.
//
// Le champ ID est opaque et n'a de sens que pour la source qui l'a produit :
// les portails ne s'accordent ni sur les identifiants ni sur le découpage. Il
// ne doit donc jamais être stocké, comparé entre sources, ni deviné — seulement
// renvoyé tel quel à la source qui l'a émis.
type ZoneAnnonce struct {
	Source string
	ID     string
	Nom    string
	Niveau NiveauZone

	CodeInsee  string // quand la source le donne : le seul pont vers DVF
	CodePostal string
	Lat, Lon   float64
}

func (z ZoneAnnonce) String() string {
	if z.CodePostal != "" {
		return z.Nom + " (" + z.CodePostal + ")"
	}
	return z.Nom
}

// ---------------------------------------------------------------------------
// Les critères
// ---------------------------------------------------------------------------

// Criteres décrit une recherche. Tous les champs sont facultatifs : le zéro
// d'un champ signifie « pas de contrainte », ce qui permet de construire une
// recherche par affectations successives sans constructeur.
//
// Une borne haute à zéro ne veut donc pas dire « surface nulle » mais « pas de
// maximum ». C'est le prix à payer pour des zero values utilisables, et il faut
// s'en souvenir en écrivant les filtres.
type Criteres struct {
	Zones []ZoneAnnonce // union : plusieurs zones élargissent la recherche
	Types []TypeBien    // vide : tous les types

	// Rayon autour d'un point, en kilomètres. Filtré localement : aucun
	// portail testé ne l'expose, et celui qui l'exposerait ne dirait pas
	// comment il le calcule.
	CentreLat, CentreLon float64
	RayonKm              float64

	PrixMin, PrixMax       float64
	SurfaceMin, SurfaceMax float64
	TerrainMin, TerrainMax float64
	PiecesMin              int
	ExclureNeuf            bool

	Tri TriAnnonces

	// Limite borne le nombre d'annonces rapportées, pagination comprise.
	// Sans elle, « tous les appartements de Nouvelle-Aquitaine » représente
	// des dizaines de milliers de requêtes.
	Limite int
}

const limiteAnnoncesParDefaut = 200

// Limitees renvoie la limite effective : un appelant qui n'en fixe pas ne doit
// pas pouvoir lancer une recherche non bornée par inadvertance.
func (c Criteres) Limitees() int {
	if c.Limite <= 0 {
		return limiteAnnoncesParDefaut
	}
	return c.Limite
}

// VeutType indique si un type de bien est demandé. Une liste vide vaut « tous ».
func (c Criteres) VeutType(t TypeBien) bool {
	if len(c.Types) == 0 {
		return true
	}
	for _, v := range c.Types {
		if v == t {
			return true
		}
	}
	return false
}

// Resume décrit la recherche en une ligne, pour l'en-tête des résultats.
func (c Criteres) Resume() string {
	var parts []string

	if len(c.Zones) > 0 {
		noms := make([]string, 0, len(c.Zones))
		for _, z := range c.Zones {
			noms = append(noms, z.Nom)
		}
		if len(noms) > 3 {
			noms = append(noms[:3], fmt.Sprintf("+%d", len(c.Zones)-3))
		}
		parts = append(parts, strings.Join(noms, ", "))
	}
	if c.RayonKm > 0 {
		parts = append(parts, fmt.Sprintf("rayon %g km", c.RayonKm))
	}
	if len(c.Types) > 0 && len(c.Types) < len(TousTypes()) {
		noms := make([]string, 0, len(c.Types))
		for _, t := range c.Types {
			noms = append(noms, t.String())
		}
		parts = append(parts, strings.Join(noms, "/"))
	}
	if c.PrixMin > 0 || c.PrixMax > 0 {
		parts = append(parts, "prix "+intervalle(c.PrixMin, c.PrixMax, "€"))
	}
	if c.SurfaceMin > 0 || c.SurfaceMax > 0 {
		parts = append(parts, "surface "+intervalle(c.SurfaceMin, c.SurfaceMax, "m²"))
	}
	if c.TerrainMin > 0 || c.TerrainMax > 0 {
		parts = append(parts, "terrain "+intervalle(c.TerrainMin, c.TerrainMax, "m²"))
	}
	if c.PiecesMin > 0 {
		parts = append(parts, fmt.Sprintf("≥ %d pièces", c.PiecesMin))
	}
	if c.ExclureNeuf {
		parts = append(parts, "ancien seulement")
	}
	if len(parts) == 0 {
		return "toute la France, sans filtre"
	}
	return strings.Join(parts, " · ")
}

func intervalle(min, max float64, unite string) string {
	switch {
	case min > 0 && max > 0:
		return fmt.Sprintf("%g–%g %s", min, max, unite)
	case max > 0:
		return fmt.Sprintf("≤ %g %s", max, unite)
	default:
		return fmt.Sprintf("≥ %g %s", min, unite)
	}
}

// ---------------------------------------------------------------------------
// Le tri
// ---------------------------------------------------------------------------

// TriAnnonces est un critère de tri. Le sens est porté à part : on veut aussi
// bien les moins chers d'abord (chercher une affaire) que les plus chers
// (regarder le haut du marché), sans doubler le nombre de constantes.
type TriAnnonces struct {
	Critere     CritereTri
	Decroissant bool
}

type CritereTri int

const (
	TriAPrix CritereTri = iota
	TriAPrixM2
	TriASurface
	TriATerrain
	TriADate
)

// LibellesTriAnnonces : l'ordre suit celui de la constante.
var LibellesTriAnnonces = []string{"prix", "prix/m²", "surface", "terrain", "date"}

func (t TriAnnonces) String() string {
	sens := "↑"
	if t.Decroissant {
		sens = "↓"
	}
	if int(t.Critere) < len(LibellesTriAnnonces) {
		return LibellesTriAnnonces[t.Critere] + " " + sens
	}
	return "tri " + sens
}

// Suivant fait défiler les critères de tri en conservant le sens.
func (t TriAnnonces) Suivant() TriAnnonces {
	t.Critere = (t.Critere + 1) % CritereTri(len(LibellesTriAnnonces))
	return t
}

// Inverse bascule le sens du tri.
func (t TriAnnonces) Inverse() TriAnnonces {
	t.Decroissant = !t.Decroissant
	return t
}

// cle extrait la valeur sur laquelle trier.
func (t TriAnnonces) cle(a Annonce) float64 {
	switch t.Critere {
	case TriAPrixM2:
		return a.PrixM2()
	case TriASurface:
		return a.Surface
	case TriATerrain:
		return a.SurfaceTerrain
	case TriADate:
		// Une date masquée se range juste avant son seuil : plus ancienne que
		// toutes les annonces datées, dans un sens comme dans l'autre. Entre
		// elles, l'ordre est inconnu — l'identité les départage.
		switch {
		case !a.Publiee.IsZero():
			return float64(a.Publiee.Unix())
		case !a.PublieeAvant.IsZero():
			return float64(a.PublieeAvant.Unix() - 1)
		}
		return 0
	default:
		return a.Prix
	}
}

// TrierAnnonces ordonne sur place.
//
// Deux précautions qui ne se voient qu'à l'usage. D'abord les valeurs absentes
// (zéro) partent systématiquement en fin de liste, quel que soit le sens : un
// tri croissant par surface qui commencerait par trente annonces sans surface
// serait inutilisable. Ensuite le tri est *stable* et départagé par l'identité,
// pour qu'une même recherche relancée rende deux fois le même ordre — sans
// quoi le curseur de l'utilisateur saute d'une ligne à l'autre.
func TrierAnnonces(as []Annonce, t TriAnnonces) {
	sort.SliceStable(as, func(i, j int) bool {
		vi, vj := t.cle(as[i]), t.cle(as[j])
		switch {
		case vi == 0 && vj == 0:
			return as[i].ID < as[j].ID
		case vi == 0:
			return false
		case vj == 0:
			return true
		case vi == vj:
			return as[i].ID < as[j].ID
		case t.Decroissant:
			return vi > vj
		default:
			return vi < vj
		}
	})
}

// ---------------------------------------------------------------------------
// Le filtrage local
// ---------------------------------------------------------------------------

// Correspond dit si une annonce satisfait les critères.
//
// Cette fonction fait doublon avec les filtres envoyés au portail, et c'est
// voulu : elle est la seule garantie. Un portail peut ignorer silencieusement
// un filtre qu'il ne comprend pas — c'est le cas mesuré des bornes de surface
// de terrain sur Bien'ici — et rendra alors des résultats hors critères sans
// aucun signal d'erreur. Filtrer une seconde fois localement coûte quelques
// microsecondes et transforme un filtre « au mieux » en filtre exact.
//
// Le corollaire : un champ absent (zéro) ne peut pas être jugé. On garde
// l'annonce plutôt que de la jeter — un terrain sans surface annoncée reste un
// terrain, et l'écarter sur un critère de surface habitable serait absurde.
func Correspond(a Annonce, c Criteres) bool {
	if !c.VeutType(a.Type) {
		return false
	}
	if c.ExclureNeuf && a.Neuf {
		return false
	}
	if !dansBornes(a.Prix, c.PrixMin, c.PrixMax) {
		return false
	}
	if !dansBornes(a.Surface, c.SurfaceMin, c.SurfaceMax) {
		return false
	}
	if !dansBornes(a.SurfaceTerrain, c.TerrainMin, c.TerrainMax) {
		return false
	}
	if c.PiecesMin > 0 && a.Pieces > 0 && a.Pieces < c.PiecesMin {
		return false
	}
	if c.RayonKm > 0 {
		if !a.Localisee() {
			return false // sans position, on ne peut pas affirmer qu'elle est dedans
		}
		if DistanceKm(c.CentreLat, c.CentreLon, a.Lat, a.Lon) > c.RayonKm {
			return false
		}
	}
	return true
}

// dansBornes : zéro signifie « non renseigné » pour la valeur et « pas de
// contrainte » pour les bornes. Les deux sens du zéro cohabitent ici, c'est
// l'endroit où il faut le dire.
func dansBornes(v, min, max float64) bool {
	if v <= 0 {
		return true // valeur inconnue : on ne l'exclut pas
	}
	if min > 0 && v < min {
		return false
	}
	if max > 0 && v > max {
		return false
	}
	return true
}

// FiltrerAnnonces applique Correspond à une liste et déduplique.
//
// La déduplication n'est pas un luxe : une même annonce est fréquemment
// republiée par plusieurs agences, et une recherche multi-zones peut ramener
// deux fois le même bien quand les zones se recouvrent. La clé est
// source+identifiant, la seule qui soit fiable ; deux annonces identiques
// venant de deux portails restent deux lignes, faute de pouvoir prouver qu'il
// s'agit du même bien.
func FiltrerAnnonces(as []Annonce, c Criteres) []Annonce {
	vues := make(map[string]bool, len(as))
	out := make([]Annonce, 0, len(as))
	for _, a := range as {
		cle := a.Source + "\x00" + a.ID
		if vues[cle] {
			continue
		}
		vues[cle] = true
		if Correspond(a, c) {
			out = append(out, a)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Distance
// ---------------------------------------------------------------------------

const rayonTerreKm = 6371.0

// DistanceKm calcule la distance orthodromique entre deux points.
//
// La projection équirectangulaire utilisée pour dessiner la carte ne convient
// pas ici : elle sert à *afficher*, avec une erreur invisible à l'écran mais
// bien réelle en kilomètres. Un rayon de recherche est une promesse faite à
// l'utilisateur, elle mérite la formule exacte.
func DistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * rayonTerreKm * math.Asin(math.Sqrt(a))
}

// ---------------------------------------------------------------------------
// Les sources
// ---------------------------------------------------------------------------

// SourceAnnonces est un portail interrogeable. Trois méthodes suffisent, et
// c'est le point : ajouter un portail ne doit toucher ni le modèle, ni les
// filtres, ni l'interface.
//
// Le contrat est délibérément faible sur Chercher : la source rend « au mieux »
// ce qui approche les critères, le cœur du programme se charge du filtrage
// exact et du tri final. Exiger l'inverse rendrait l'interface impossible à
// implémenter — aucun portail n'expose la même grille de filtres.
type SourceAnnonces interface {
	// Nom identifie la source dans les annonces qu'elle produit.
	Nom() string

	// ResoudreZone traduit une saisie libre (« Bayonne », « Landes »,
	// « Nouvelle-Aquitaine ») en zones utilisables par cette source.
	ResoudreZone(ctx context.Context, requete string) ([]ZoneAnnonce, error)

	// Chercher renvoie les annonces approchant les critères, et le nombre
	// total annoncé par le portail — qui n'est presque jamais le nombre
	// d'annonces rendues, pagination et filtrage local obligent.
	Chercher(ctx context.Context, c Criteres) ([]Annonce, int, error)
}

// sources est le registre. Une map plutôt qu'un slice : on veut retrouver une
// source par son nom pour lui redonner les ZoneAnnonce qu'elle a émises.
var sources = map[string]SourceAnnonces{}
var ordreSources []string

// EnregistrerSource ajoute un portail. Appelée depuis un init() dans le fichier
// de chaque adaptateur : le registre reste ignorant de ce qu'il contient, et
// supprimer un fichier suffit à retirer un portail.
func EnregistrerSource(s SourceAnnonces) {
	if _, deja := sources[s.Nom()]; !deja {
		ordreSources = append(ordreSources, s.Nom())
	}
	sources[s.Nom()] = s
}

// Sources énumère les portails enregistrés, dans l'ordre d'enregistrement.
func Sources() []SourceAnnonces {
	out := make([]SourceAnnonces, 0, len(ordreSources))
	for _, nom := range ordreSources {
		out = append(out, sources[nom])
	}
	return out
}

// SourceParNom retrouve un portail.
func SourceParNom(nom string) (SourceAnnonces, bool) {
	s, ok := sources[nom]
	return s, ok
}

// ---------------------------------------------------------------------------
// La recherche, toutes sources confondues
// ---------------------------------------------------------------------------

// ResultatAnnonces rend compte d'une recherche complète.
type ResultatAnnonces struct {
	Annonces []Annonce
	Total    int              // somme des totaux annoncés par les portails
	Erreurs  map[string]error // par source : une panne n'annule pas les autres
}

// ChercherAnnonces interroge toutes les sources concernées, filtre, trie et
// borne.
//
// Le découpage par source des critères est important : chaque source ne reçoit
// que les zones qu'elle a elle-même résolues. Envoyer à Bien'ici un
// identifiant de zone venu d'ailleurs produirait au mieux une erreur, au pire
// des résultats silencieusement faux — le pire étant ici bien plus probable.
//
// Une source en panne ne fait pas échouer la recherche : ses erreurs sont
// collectées et affichées, les autres résultats restent exploitables. C'est le
// bon compromis pour un agrégateur — l'inverse rendrait le programme aussi
// fiable que le moins fiable de ses portails.
func ChercherAnnonces(ctx context.Context, c Criteres) (ResultatAnnonces, error) {
	res := ResultatAnnonces{Erreurs: map[string]error{}}

	parSource := repartirZones(c.Zones)
	if len(c.Zones) == 0 {
		// Sans zone, toutes les sources sont interrogées sur la France entière.
		for _, s := range Sources() {
			parSource[s.Nom()] = nil
		}
	}
	if len(parSource) == 0 {
		return res, fmt.Errorf("aucune source disponible pour les zones demandées")
	}

	for nom, zones := range parSource {
		s, ok := SourceParNom(nom)
		if !ok {
			res.Erreurs[nom] = fmt.Errorf("source inconnue")
			continue
		}
		sous := c
		sous.Zones = zones

		as, total, err := s.Chercher(ctx, sous)
		if err != nil {
			res.Erreurs[nom] = err
			continue
		}
		res.Total += total
		res.Annonces = append(res.Annonces, as...)
	}

	res.Annonces = FiltrerAnnonces(res.Annonces, c)
	TrierAnnonces(res.Annonces, c.Tri)
	if n := c.Limitees(); len(res.Annonces) > n {
		res.Annonces = res.Annonces[:n]
	}

	// Toutes les sources en échec : là, c'est une vraie erreur.
	if len(res.Annonces) == 0 && len(res.Erreurs) > 0 && len(res.Erreurs) == len(parSource) {
		for _, err := range res.Erreurs {
			return res, err
		}
	}
	return res, nil
}

func repartirZones(zones []ZoneAnnonce) map[string][]ZoneAnnonce {
	out := map[string][]ZoneAnnonce{}
	for _, z := range zones {
		out[z.Source] = append(out[z.Source], z)
	}
	return out
}

// ResoudreZones interroge toutes les sources pour une saisie libre. C'est ce
// qui alimente l'autocomplétion de l'écran de recherche : l'utilisateur tape un
// nom, chaque portail propose ses zones, et il choisit.
func ResoudreZones(ctx context.Context, requete string) ([]ZoneAnnonce, error) {
	requete = strings.TrimSpace(requete)
	if requete == "" {
		return nil, nil
	}
	var out []ZoneAnnonce
	var derniere error
	for _, s := range Sources() {
		zs, err := s.ResoudreZone(ctx, requete)
		if err != nil {
			derniere = fmt.Errorf("%s : %w", s.Nom(), err)
			continue
		}
		out = append(out, zs...)
	}
	if len(out) == 0 && derniere != nil {
		return nil, derniere
	}
	// L'ordre est celui rendu par les portails, et il ne faut pas y toucher.
	// La tentation est de remonter les zones les plus larges — « Landes »
	// désigne bien plus souvent le département que la commune homonyme. Mais
	// essaie sur « Bayonne » : le classement par échelon fait remonter un
	// département vaguement approchant devant la ville cherchée. Les portails
	// classent par pertinence, ce qu'un tri sur un seul champ ne sait pas
	// reproduire. Ne réordonne pas un classement que tu ne sais pas refaire.
	return out, nil
}

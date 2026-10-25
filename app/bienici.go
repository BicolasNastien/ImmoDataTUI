package main

// ---------------------------------------------------------------------------
// Adaptateur Bien'ici
// ---------------------------------------------------------------------------
//
// Pourquoi Bien'ici et pas leboncoin. Le premier réflexe est d'aller chercher
// les annonces là où il y en a le plus. Vérifie avant de coder : leboncoin
// répond 403 à toute requête qui ne vient pas d'un navigateur — une redirection
// vers un service anti-robot — et son robots.txt commence par « It's forbidden
// to use search robots or other automatic methods to access Leboncoin.fr ».
// Ce n'est pas un obstacle technique à contourner, c'est un refus explicite.
//
// Bien'ici, lui, sert le même JSON à son propre site et à qui le demande :
//
//	https://www.bienici.com/realEstateAds.json?filters=<json urlencodé>
//	https://res.bienici.com/suggest.json?q=<texte>
//
// Ce n'est pas une API contractuelle pour autant : elle peut changer sans
// préavis, et c'est une raison de plus pour l'isoler derrière SourceAnnonces.
// Le jour où elle casse, un seul fichier est à réécrire.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	urlAnnoncesBienici = "https://www.bienici.com/realEstateAds.json"
	urlSuggestBienici  = "https://res.bienici.com/suggest.json"
	urlFicheBienici    = "https://www.bienici.com/annonce/"

	// Taille de page. Le portail en accepte de plus grosses, mais une page
	// modeste garde chaque requête courte et rend l'annulation réactive : on
	// abandonne entre deux pages, pas au milieu d'un téléchargement.
	pageBienici = 50

	// Garde-fou : même avec une limite haute, on ne pagine pas indéfiniment
	// sur une recherche mal bornée.
	maxPagesBienici = 20

	// Le portail est gratuit et sert ses propres visiteurs : on ne le martèle
	// pas. Une courte pause entre deux pages coûte peu et se voit beaucoup
	// dans leurs journaux.
	pauseBienici = 150 * time.Millisecond
)

// clientBienici : un délai court, ces réponses font quelques centaines de
// kilo-octets. Une requête qui traîne est une requête qui ne répondra pas.
var clientBienici = &http.Client{Timeout: 30 * time.Second}

// entetesBienici : un User-Agent identifiable est la politesse minimale, et
// beaucoup de serveurs refusent l'absence d'en-tête plutôt que sa valeur.
const uaBienici = "dvf-tui (+https://github.com/) Go-http-client"

type sourceBienici struct{}

func init() { EnregistrerSource(sourceBienici{}) }

func (sourceBienici) Nom() string { return "Bien'ici" }

// ---------------------------------------------------------------------------
// Résolution géographique
// ---------------------------------------------------------------------------

// suggestionBienici est une entrée de l'autocomplétion du portail.
type suggestionBienici struct {
	ID          string   `json:"id"`
	Nom         string   `json:"name"`
	Type        string   `json:"type"`
	ZoneIDs     []string `json:"zoneIds"`
	InseeCodes  []string `json:"insee_codes"`
	PostalCodes []string `json:"postalCodes"`
	Boite       struct {
		Ouest float64 `json:"west"`
		Sud   float64 `json:"south"`
		Est   float64 `json:"east"`
		Nord  float64 `json:"north"`
	} `json:"boundingBox"`
}

func (s suggestionBienici) niveau() NiveauZone {
	switch s.Type {
	case "department":
		return ZoneDepartement
	case "region":
		return ZoneRegion
	default:
		return ZoneCommune
	}
}

// ResoudreZone traduit une saisie libre en zones Bien'ici.
//
// Deux chemins, et c'est là qu'est tout l'intérêt de la méthode. Les communes
// et les départements sont dans l'autocomplétion du portail. Les régions n'y
// sont pas — vérifie-le, « Nouvelle-Aquitaine » ne rend que des communes
// homonymes. On les reconstitue donc avec les contours déjà embarqués dans le
// programme : une région est l'union de ses départements, et le portail sait
// combiner plusieurs zones. Une lacune de la source se comble avec une donnée
// qu'on possède déjà.
func (s sourceBienici) ResoudreZone(ctx context.Context, requete string) ([]ZoneAnnonce, error) {
	requete = strings.TrimSpace(requete)
	if requete == "" {
		return nil, nil
	}

	if zones, ok, err := s.zonesDeRegion(ctx, requete); ok {
		return zones, err
	}

	sugs, err := s.suggerer(ctx, requete)
	if err != nil {
		return nil, err
	}

	var out []ZoneAnnonce
	vues := map[string]bool{}
	for _, sg := range sugs {
		if len(sg.ZoneIDs) == 0 {
			continue
		}
		id := sg.ZoneIDs[0]
		if vues[id] {
			continue // « alias-city » et « city » désignent souvent la même zone
		}
		vues[id] = true

		z := ZoneAnnonce{
			Source: s.Nom(),
			ID:     id,
			Nom:    premierNom(sg.Nom),
			Niveau: sg.niveau(),
			Lat:    (sg.Boite.Sud + sg.Boite.Nord) / 2,
			Lon:    (sg.Boite.Ouest + sg.Boite.Est) / 2,
		}
		if len(sg.InseeCodes) > 0 {
			z.CodeInsee = sg.InseeCodes[0]
		}
		if len(sg.PostalCodes) > 0 {
			z.CodePostal = sg.PostalCodes[0]
		}
		out = append(out, z)
	}
	return out, nil
}

// premierNom : le portail renvoie parfois les variantes linguistiques d'une
// commune sur une seule ligne (« Bayonne/Bayona/Baiona »). On garde la
// première, les autres n'apportent rien dans un tableau.
func premierNom(nom string) string {
	if i := strings.IndexByte(nom, '/'); i > 0 {
		return nom[:i]
	}
	return nom
}

// zonesDeRegion tente d'interpréter la requête comme un nom de région. Le
// booléen dit si c'en était une : sans lui, l'appelant ne saurait pas
// distinguer « région inconnue » de « région sans département », et repasserait
// par l'autocomplétion pour rien.
func (s sourceBienici) zonesDeRegion(ctx context.Context, requete string) ([]ZoneAnnonce, bool, error) {
	regions, err := Regions()
	if err != nil {
		return nil, false, nil // pas de contours embarqués : on retombe sur le portail
	}
	var region Zone
	for _, r := range regions {
		if egalSansAccent(r.Nom, requete) {
			region = r
			break
		}
	}
	if region.Code == "" {
		return nil, false, nil
	}

	deps, err := DepartementsDe(region.Code)
	if err != nil || len(deps) == 0 {
		return nil, true, fmt.Errorf("région %s : aucun département connu", region.Nom)
	}

	// Un aller-retour par département : treize au maximum, en parallèle. Le
	// sémaphore n'est pas une optimisation, c'est une politesse — treize
	// requêtes simultanées sur un serveur qu'on ne paye pas, non.
	trouvees := make([]ZoneAnnonce, len(deps))
	var wg sync.WaitGroup
	jetonsGeo := make(chan struct{}, 3)

	for i, d := range deps {
		wg.Add(1)
		go func(i int, d Zone) {
			defer wg.Done()
			jetonsGeo <- struct{}{}
			defer func() { <-jetonsGeo }()

			sugs, err := s.suggerer(ctx, d.Nom)
			if err != nil {
				return
			}
			for _, sg := range sugs {
				if sg.Type == "department" && len(sg.ZoneIDs) > 0 {
					trouvees[i] = ZoneAnnonce{
						Source: s.Nom(),
						ID:     sg.ZoneIDs[0],
						Nom:    d.Nom,
						Niveau: ZoneDepartement,
					}
					return
				}
			}
		}(i, d)
	}
	wg.Wait()

	var out []ZoneAnnonce
	for _, z := range trouvees {
		if z.ID != "" {
			out = append(out, z)
		}
	}
	if len(out) == 0 {
		return nil, true, fmt.Errorf("région %s : le portail n'a reconnu aucun de ses départements", region.Nom)
	}
	return out, true, nil
}

func (s sourceBienici) suggerer(ctx context.Context, requete string) ([]suggestionBienici, error) {
	u := urlSuggestBienici + "?" + url.Values{"q": {requete}}.Encode()
	brut, err := getBienici(ctx, u)
	if err != nil {
		return nil, err
	}
	var sugs []suggestionBienici
	if err := json.Unmarshal(brut, &sugs); err != nil {
		return nil, fmt.Errorf("autocomplétion « %s » : %w", requete, err)
	}
	return sugs, nil
}

// ---------------------------------------------------------------------------
// Recherche
// ---------------------------------------------------------------------------

// filtresBienici est le corps de requête du portail, sérialisé en JSON puis
// placé dans le paramètre d'URL `filters`.
//
// Les `omitempty` ne sont pas cosmétiques : envoyer `"minArea": 0` n'est pas la
// même chose que ne rien envoyer, et le portail interprète le zéro. C'est le
// point de contact entre nos zero values « pas de contrainte » et une API qui
// ne connaît pas cette convention — la traduction se fait ici, une fois.
type filtresBienici struct {
	Taille      int           `json:"size"`
	Depuis      int           `json:"from"`
	Page        int           `json:"page"`
	TypeFiltre  string        `json:"filterType"` // "buy" : les ventes, pas les locations
	EnVente     []bool        `json:"onTheMarket"`
	TypesBien   []string      `json:"propertyType,omitempty"`
	TriPar      string        `json:"sortBy"`
	SensTri     string        `json:"sortOrder"`
	PrixMin     float64       `json:"minPrice,omitempty"`
	PrixMax     float64       `json:"maxPrice,omitempty"`
	SurfaceMin  float64       `json:"minArea,omitempty"`
	SurfaceMax  float64       `json:"maxArea,omitempty"`
	PiecesMin   int           `json:"minRooms,omitempty"`
	Zones       *zonesBienici `json:"zoneIdsByTypes,omitempty"`
	ExclureNeuf *bool         `json:"newProperty,omitempty"`
}

type zonesBienici struct {
	ZoneIDs []string `json:"zoneIds"`
}

// construireFiltres traduit nos critères en filtres du portail.
//
// Trois critères ne passent volontairement pas la frontière : les bornes de
// surface de terrain, que le portail accepte et ignore (mesure-le : demande un
// terrain de plus de 500 m², il te rendra un lot de 328 m²), le rayon, qu'il
// n'expose pas, et le tri par prix au m². Ils sont appliqués au retour, par le
// cœur du programme. Envoyer un filtre en croyant qu'il agit est pire que ne
// pas l'envoyer : on cesse alors de vérifier.
func construireFiltres(c Criteres, page int, tri TriAnnonces) filtresBienici {
	f := filtresBienici{
		Taille:     pageBienici,
		Depuis:     page * pageBienici,
		Page:       page + 1,
		TypeFiltre: "buy",
		EnVente:    []bool{true},
		PrixMin:    c.PrixMin,
		PrixMax:    c.PrixMax,
		SurfaceMin: c.SurfaceMin,
		SurfaceMax: c.SurfaceMax,
		PiecesMin:  c.PiecesMin,
	}

	f.TriPar, f.SensTri = triBienici(tri)

	for _, t := range c.Types {
		if v, ok := typeVersBienici[t]; ok {
			f.TypesBien = append(f.TypesBien, v...)
		}
	}
	if len(c.Zones) > 0 {
		z := &zonesBienici{}
		for _, zone := range c.Zones {
			z.ZoneIDs = append(z.ZoneIDs, zone.ID)
		}
		f.Zones = z
	}
	if c.ExclureNeuf {
		faux := false
		f.ExclureNeuf = &faux
	}
	return f
}

// typeVersBienici : notre nomenclature à trois entrées vers la leur, qui en a
// une quinzaine. Une maison de maître, un château et un chalet sont des maisons
// pour qui compare des prix ; les distinguer serait de la fausse précision.
var typeVersBienici = map[TypeBien][]string{
	BienMaison:      {"house", "villa", "chalet", "castle", "townhouse", "manor", "farmhouse"},
	BienAppartement: {"flat", "loft", "duplex", "penthouse", "studio"},
	BienTerrain:     {"terrain", "land"},
}

// typeDepuisBienici : la traduction inverse. Tout ce qui n'y figure pas —
// parking, cave, local commercial, mais aussi « programme », qui désigne un
// immeuble neuf entier et non un bien — devient BienInconnu et sera écarté par
// le filtre de type.
var typeDepuisBienici = map[string]TypeBien{
	"house": BienMaison, "villa": BienMaison, "chalet": BienMaison,
	"castle": BienMaison, "townhouse": BienMaison, "manor": BienMaison,
	"farmhouse": BienMaison, "mas": BienMaison, "pavilion": BienMaison,
	"flat": BienAppartement, "loft": BienAppartement, "duplex": BienAppartement,
	"penthouse": BienAppartement, "studio": BienAppartement,
	"terrain": BienTerrain, "land": BienTerrain,
}

// triBienici traduit notre tri. Le portail ne connaît ni le prix au mètre carré
// ni la surface de terrain : on lui demande alors le prix, et on retrie
// localement.
//
// Cette dégradation a une conséquence qu'il faut assumer : trier par prix au m²
// n'ordonne que les annonces rapportées, pas le marché entier. Le premier
// résultat est « le meilleur prix au m² parmi les N premiers par prix », pas
// « le meilleur du département ». Un tri sur un échantillon reste un tri sur un
// échantillon, et c'est à l'affichage de ne pas le faire croire.
func triBienici(t TriAnnonces) (par, sens string) {
	sens = "asc"
	if t.Decroissant {
		sens = "desc"
	}
	switch t.Critere {
	case TriASurface:
		return "surface", sens
	case TriADate:
		return "publicationDate", sens
	default:
		return "price", sens
	}
}

// ---------------------------------------------------------------------------
// Décodage
// ---------------------------------------------------------------------------

// nombreBienici absorbe un champ qui est tantôt un nombre, tantôt une liste.
//
// Ce n'est pas une bizarrerie gratuite : une annonce de programme neuf porte
// plusieurs lots, donc plusieurs prix et plusieurs surfaces, dans les mêmes
// champs qu'un bien unique. Un `float64` classique fait échouer tout le
// décodage de la page sur une seule annonce de ce genre — c'est le genre de
// bogue qui n'apparaît qu'en production, sur la commune où il y a un chantier.
//
// On retient la plus petite valeur : pour un programme, c'est le ticket
// d'entrée, la seule qui soit comparable à un prix de bien unique.
type nombreBienici float64

func (n *nombreBienici) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '[' {
		var vs []float64
		if err := json.Unmarshal(b, &vs); err != nil {
			return err
		}
		if len(vs) == 0 {
			return nil
		}
		min := vs[0]
		for _, v := range vs[1:] {
			if v < min {
				min = v
			}
		}
		*n = nombreBienici(min)
		return nil
	}
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		// Une valeur inattendue sur un champ facultatif ne doit pas faire
		// tomber la page entière : on la traite comme absente.
		return nil
	}
	*n = nombreBienici(v)
	return nil
}

func (n nombreBienici) f() float64 { return float64(n) }

// entierBienici : même problème, même remède, pour le nombre de pièces.
type entierBienici int

func (e *entierBienici) UnmarshalJSON(b []byte) error {
	var n nombreBienici
	if err := n.UnmarshalJSON(b); err != nil {
		return err
	}
	*e = entierBienici(n)
	return nil
}

type reponseBienici struct {
	Total    int              `json:"total"`
	Annonces []annonceBienici `json:"realEstateAds"`
}

type annonceBienici struct {
	ID             string        `json:"id"`
	Titre          string        `json:"title"`
	TypeBien       string        `json:"propertyType"`
	Prix           nombreBienici `json:"price"`
	Surface        nombreBienici `json:"surfaceArea"`
	SurfaceTerrain nombreBienici `json:"landSurfaceArea"`
	Pieces         entierBienici `json:"roomsQuantity"`
	Commune        string        `json:"city"`
	CodePostal     string        `json:"postalCode"`
	Departement    string        `json:"departmentCode"`
	Neuf           bool          `json:"newProperty"`
	ParPro         bool          `json:"adCreatedByPro"`
	Publiee        string        `json:"publicationDate"`
	PrixNetVendeur nombreBienici `json:"priceWithoutFees"`
	Honoraires     string        `json:"feesChargedTo"` // "purchaser" ou "seller"
	TypeAnnonce    string        `json:"adType"`        // "buy", "lifeAnnuitySale"…

	Flou struct {
		Position struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"position"`
	} `json:"blurInfo"`
}

func (a annonceBienici) versAnnonce(source string) Annonce {
	out := Annonce{
		Source:         source,
		ID:             a.ID,
		URL:            urlFicheBienici + a.ID,
		Titre:          strings.TrimSpace(a.Titre),
		Type:           typeDepuisBienici[a.TypeBien],
		Prix:           a.Prix.f(),
		Surface:        a.Surface.f(),
		SurfaceTerrain: a.SurfaceTerrain.f(),
		Pieces:         int(a.Pieces),
		Commune:        a.Commune,
		CodePostal:     a.CodePostal,
		Departement:    a.Departement,
		Lat:            a.Flou.Position.Lat,
		Lon:            a.Flou.Position.Lon,
		Neuf:           a.Neuf,
		Pro:            a.ParPro,
		Viager:         a.TypeAnnonce == "lifeAnnuitySale",
	}
	// Le prix hors honoraires n'est celui de l'acte que si l'acheteur paie
	// l'agence. Quand c'est le vendeur, l'acte porte le prix affiché, et le
	// portail renvoie d'ailleurs un prix hors honoraires vide ou nul.
	if a.Honoraires == "purchaser" && a.PrixNetVendeur.f() > 0 {
		out.PrixActe = a.PrixNetVendeur.f()
	}
	if t, err := time.Parse(time.RFC3339, a.Publiee); err == nil {
		out.Publiee = t
	}
	return out
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// Chercher pagine jusqu'à satisfaire la limite demandée.
//
// La condition d'arrêt n'est pas « j'ai atteint la limite » mais « j'ai atteint
// la limite, OU la page est vide, OU j'ai épuisé le total annoncé, OU le
// contexte est annulé, OU j'ai paginé trop longtemps ». Une seule de ces
// conditions oubliée et tu boucles indéfiniment sur un portail qui répond
// gentiment une page vide à l'infini. Écris-les toutes avant la boucle.
//
// On demande plus d'annonces que la limite finale, parce que le filtrage local
// (rayon, terrain) va en écarter une partie : sans cette marge, une recherche
// dans un rayon de 5 km rendrait trois résultats sur les deux cents espérés.
func (s sourceBienici) Chercher(ctx context.Context, c Criteres) ([]Annonce, int, error) {
	voulues := c.Limitees()
	if c.RayonKm > 0 || c.TerrainMin > 0 || c.TerrainMax > 0 {
		voulues *= 3
	}

	var (
		out   []Annonce
		total int
	)
	for page := 0; page < maxPagesBienici; page++ {
		select {
		case <-ctx.Done():
			return out, total, ctx.Err()
		default:
		}
		if page > 0 {
			time.Sleep(pauseBienici)
		}

		rep, err := s.page(ctx, c, page)
		if err != nil {
			// Une page en échec après des pages réussies : on rend ce qu'on a
			// plutôt que de tout perdre. L'utilisateur préfère cent annonces à
			// un message d'erreur.
			if len(out) > 0 {
				return out, total, nil
			}
			return nil, 0, err
		}
		total = rep.Total

		if len(rep.Annonces) == 0 {
			break
		}
		for _, a := range rep.Annonces {
			out = append(out, a.versAnnonce(s.Nom()))
		}
		if len(out) >= voulues || len(out) >= total {
			break
		}
	}
	return out, total, nil
}

func (s sourceBienici) page(ctx context.Context, c Criteres, page int) (reponseBienici, error) {
	var rep reponseBienici

	f := construireFiltres(c, page, c.Tri)
	corps, err := json.Marshal(f)
	if err != nil {
		return rep, fmt.Errorf("filtres Bien'ici : %w", err)
	}

	u := urlAnnoncesBienici + "?" + url.Values{"filters": {string(corps)}}.Encode()
	brut, err := getBienici(ctx, u)
	if err != nil {
		return rep, err
	}
	if err := json.Unmarshal(brut, &rep); err != nil {
		return rep, fmt.Errorf("annonces Bien'ici (page %d) : %w", page+1, err)
	}
	return rep, nil
}

// getBienici exécute la requête et vérifie le statut. Le contrôle explicite du
// code de retour n'est pas facultatif : net/http ne considère pas un 403 comme
// une erreur, et sans ce test on décoderait une page HTML de refus en croyant
// tenir des annonces.
func getBienici(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", uaBienici)
	req.Header.Set("Accept", "application/json")

	resp, err := clientBienici.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Bien'ici injoignable : %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bien'ici : statut %s", resp.Status)
	}
	return lireTout(resp.Body)
}

// egalSansAccent compare deux libellés administratifs sans se soucier de la
// casse, des accents ni des séparateurs. « Nouvelle Aquitaine », « nouvelle-
// aquitaine » et « Nouvelle-Aquitaine » désignent la même région, et personne
// ne tape le tiret au bon endroit.
func egalSansAccent(a, b string) bool { return clefLibelle(a) == clefLibelle(b) }

func clefLibelle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r2, ok := sansAccent[r]; ok {
			r = r2
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sansAccent couvre les diacritiques présents dans les noms de collectivités
// françaises. Une table explicite plutôt qu'une dépendance de normalisation
// Unicode : le domaine est fermé et connu, et il tient en dix lignes.
var sansAccent = map[rune]rune{
	'à': 'a', 'â': 'a', 'ä': 'a', 'á': 'a',
	'ç': 'c',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'î': 'i', 'ï': 'i', 'í': 'i',
	'ô': 'o', 'ö': 'o', 'ó': 'o', 'õ': 'o',
	'ù': 'u', 'û': 'u', 'ü': 'u', 'ú': 'u',
	'ÿ': 'y', 'ñ': 'n',
}

// entierOuZero est utilisé par l'écran de saisie : un champ vide vaut « pas de
// contrainte », pas une erreur à afficher.
func entierOuZero(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, " ", ""))
	s = strings.ReplaceAll(s, ",", ".")
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Contours administratifs
// ---------------------------------------------------------------------------

// Zone est une entité administrative dessinable : un code, un nom et des
// contours. Les contours sont des anneaux fermés en coordonnées à plat
// (lon, lat, lon, lat…) — un slice plat plutôt qu'un slice de points, parce que
// c'est ce que le rendu consomme et que ça divise par deux le nombre
// d'allocations au décodage.
type Zone struct {
	Code     string      `json:"code"`
	Nom      string      `json:"nom"`
	Region   string      `json:"region,omitempty"` // pour un département : sa région
	Contours [][]float64 `json:"c"`
}

// geodata embarque les contours simplifiés des régions et des départements.
// L'API Découpage administratif ne sert pas de géométrie à ces échelons — et
// même si elle le faisait, un premier écran qui dépend du réseau est un premier
// écran qui peut échouer. Voir geodata/generer.py pour la fabrication.
//
//go:embed geodata/regions.json geodata/departements.json
var geodata embed.FS

func chargerZonesEmbarquees(nom string) ([]Zone, error) {
	b, err := geodata.ReadFile("geodata/" + nom)
	if err != nil {
		return nil, fmt.Errorf("contours embarqués %s : %w", nom, err)
	}
	var zones []Zone
	if err := json.Unmarshal(b, &zones); err != nil {
		return nil, fmt.Errorf("contours embarqués %s : %w", nom, err)
	}
	return zones, nil
}

// Regions renvoie les 13 régions métropolitaines et les 5 régions d'outre-mer.
func Regions() ([]Zone, error) { return chargerZonesEmbarquees("regions.json") }

// DepartementsDe renvoie les départements d'une région donnée.
func DepartementsDe(codeRegion string) ([]Zone, error) {
	tous, err := chargerZonesEmbarquees("departements.json")
	if err != nil {
		return nil, err
	}
	var out []Zone
	for _, d := range tous {
		if d.Region == codeRegion {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

// DepartementParCode retrouve un département dans les contours embarqués. Une
// recherche d'adresse ne donne qu'un code INSEE : c'est par là qu'on remonte au
// département dont il faut télécharger les millésimes.
func DepartementParCode(code string) (Zone, bool) {
	tous, err := chargerZonesEmbarquees("departements.json")
	if err != nil {
		return Zone{}, false
	}
	for _, d := range tous {
		if d.Code == code {
			return d, true
		}
	}
	return Zone{}, false
}

// EstOutreMer indique si une région est hors métropole. Ces régions sont
// dessinées en cartouches séparés : à l'échelle, la Guyane est à 7 000 km et
// écraserait la métropole sur trois caractères.
func EstOutreMer(codeRegion string) bool {
	switch codeRegion {
	case "01", "02", "03", "04", "06":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Communes : contours récupérés à la demande
// ---------------------------------------------------------------------------

const apiGeo = "https://geo.api.gouv.fr"

// clientGeo a un délai de garde : le client HTTP par défaut n'en a aucun, une
// connexion qui pend bloquerait l'interface indéfiniment.
var clientGeo = &http.Client{Timeout: 60 * time.Second}

// apiCommune est la forme de réponse de l'API Découpage administratif.
type apiCommune struct {
	Nom     string `json:"nom"`
	Code    string `json:"code"`
	Contour struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"contour"`
}

// anneaux convertit une géométrie GeoJSON en anneaux extérieurs à plat.
// Les trous sont ignorés : aucune commune n'en a d'assez grand pour se voir.
func (c apiCommune) anneaux() [][]float64 {
	aplatir := func(ring [][]float64) []float64 {
		out := make([]float64, 0, len(ring)*2)
		for _, p := range ring {
			if len(p) >= 2 {
				out = append(out, p[0], p[1])
			}
		}
		return out
	}

	switch c.Contour.Type {
	case "Polygon":
		var poly [][][]float64
		if json.Unmarshal(c.Contour.Coordinates, &poly) != nil || len(poly) == 0 {
			return nil
		}
		return [][]float64{aplatir(poly[0])}

	case "MultiPolygon":
		var multi [][][][]float64
		if json.Unmarshal(c.Contour.Coordinates, &multi) != nil {
			return nil
		}
		var out [][]float64
		for _, poly := range multi {
			if len(poly) > 0 {
				out = append(out, aplatir(poly[0]))
			}
		}
		return out
	}
	return nil
}

// CommunesDe renvoie les communes d'un département avec leurs contours.
// La réponse de l'API est mise en cache sur disque : le découpage communal ne
// bouge quasiment jamais, et le fichier pèse plusieurs mégaoctets.
func CommunesDe(codeDep string) ([]Zone, error) {
	cache, err := cheminCache("geo", "communes-"+codeDep+".json")
	if err != nil {
		return nil, err
	}

	brut, err := os.ReadFile(cache)
	if err != nil {
		url := fmt.Sprintf("%s/departements/%s/communes?fields=nom,code,contour&format=json&geometry=contour",
			apiGeo, codeDep)
		brut, err = getOctets(url)
		if err != nil {
			return nil, fmt.Errorf("contours des communes du %s : %w", codeDep, err)
		}
		if err := ecrireAtomique(cache, brut); err != nil {
			return nil, err
		}
	}

	var communes []apiCommune
	if err := json.Unmarshal(brut, &communes); err != nil {
		os.Remove(cache) // cache corrompu : on le jette pour ne pas s'y enfermer
		return nil, fmt.Errorf("contours des communes du %s : %w", codeDep, err)
	}

	var zones []Zone
	for _, c := range communes {
		anneaux := c.anneaux()
		if len(anneaux) == 0 {
			continue
		}
		zones = append(zones, Zone{Code: c.Code, Nom: c.Nom, Contours: anneaux})
	}
	sort.Slice(zones, func(i, j int) bool { return zones[i].Nom < zones[j].Nom })
	return zones, nil
}

// getOctets exécute un GET et vérifie le statut. Un 404 n'est pas une erreur
// pour net/http : sans cette vérification on récupérerait une page d'erreur
// HTML en croyant tenir des données.
func getOctets(url string) ([]byte, error) {
	resp, err := clientGeo.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s : statut %s", url, resp.Status)
	}
	return lireTout(resp.Body)
}

// ---------------------------------------------------------------------------
// Géométrie
// ---------------------------------------------------------------------------

// Cadre est une emprise géographique en degrés.
type Cadre struct {
	LonMin, LonMax, LatMin, LatMax float64
}

func (c Cadre) valide() bool { return c.LonMax > c.LonMin && c.LatMax > c.LatMin }

// CadreDe calcule l'emprise d'un ensemble de zones.
func CadreDe(zones []Zone) Cadre {
	c := Cadre{LonMin: math.Inf(1), LonMax: math.Inf(-1), LatMin: math.Inf(1), LatMax: math.Inf(-1)}
	for _, z := range zones {
		for _, anneau := range z.Contours {
			for i := 0; i+1 < len(anneau); i += 2 {
				lon, lat := anneau[i], anneau[i+1]
				c.LonMin = math.Min(c.LonMin, lon)
				c.LonMax = math.Max(c.LonMax, lon)
				c.LatMin = math.Min(c.LatMin, lat)
				c.LatMax = math.Max(c.LatMax, lat)
			}
		}
	}
	return c
}

// Centre renvoie le centre du cadre.
func (c Cadre) Centre() (float64, float64) {
	return (c.LonMin + c.LonMax) / 2, (c.LatMin + c.LatMax) / 2
}

// NormaliserNom transforme un nom de commune en nom de dossier utilisable.
func NormaliserNom(nom string) string {
	remplacements := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "'", "-", " ", "-")
	return strings.Trim(remplacements.Replace(nom), "-")
}

func cheminCache(elements ...string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("répertoire de cache introuvable : %w", err)
	}
	chemin := filepath.Join(append([]string{base, "immodata"}, elements...)...)
	if err := os.MkdirAll(filepath.Dir(chemin), 0o755); err != nil {
		return "", fmt.Errorf("création du cache : %w", err)
	}
	return chemin, nil
}

// RegionsAvecDonnees traduit un ensemble de départements téléchargés en
// ensemble de régions, pour teinter la carte de France selon ce qu'on possède.
func RegionsAvecDonnees(deps map[string]bool) map[string]bool {
	out := map[string]bool{}
	if len(deps) == 0 {
		return out
	}
	tous, err := chargerZonesEmbarquees("departements.json")
	if err != nil {
		return out
	}
	for _, d := range tous {
		if deps[d.Code] {
			out[d.Region] = true
		}
	}
	return out
}

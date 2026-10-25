package main

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Repli sur le fichier brut de la DGFiP
// ---------------------------------------------------------------------------
//
// geo-dvf n'est qu'un retraitement : Etalab reprend le fichier « Demandes de
// valeurs foncières » publié par la DGFiP, le découpe par département, le
// nettoie et le géolocalise. Ce retraitement arrive des semaines après la
// publication d'origine. Entre les deux, le millésime le plus récent n'existe
// que dans le fichier brut.
//
// Quand geo-dvf répond 404, on va donc chercher le fichier brut, on n'en garde
// que le département demandé, et on le réécrit au format geo-dvf dans le cache.
// Le reste du programme ne voit pas la différence, à deux manques près : le
// fichier brut ne contient ni coordonnées ni identifiant de mutation. L'archive
// est marquée provisoire : dès que geo-dvf publie le millésime, elle est
// remplacée.
//
// Le fichier brut diffère sur presque tout : une seule archive zip pour toute
// la France, des colonnes aux noms français, le séparateur « | », des dates
// jj/mm/aaaa, des décimales à virgule, un code commune amputé du département.

const (
	apiDGFiP           = "https://www.data.gouv.fr/api/1/datasets/demandes-de-valeurs-foncieres/"
	suffixeProvisoire  = ".provisoire"
	fichierProvisoire  = ".provisoire" // dans un dossier millésime, comme fichierPublication
	separateurDGFiP    = "|"
	tailleLigneMaxBrut = 1 << 20
)

// Les noms de fichiers suivent la forme valeursfoncieres-2026-s1.txt.zip. Le
// suffixe de semestre n'apparaît que pour une année incomplète.
var motifFichierDGFiP = regexp.MustCompile(`^valeursfoncieres-(\d{4})(-s[12])?\.txt\.zip$`)

// verrouDGFiP sérialise les accès au fichier national : deux départements
// demandés en même temps ne doivent pas le télécharger deux fois.
var verrouDGFiP sync.Mutex

// enteteGeoDVF : les colonnes, dans l'ordre, du format qu'on reproduit.
var enteteGeoDVF = []string{
	"id_mutation", "date_mutation", "numero_disposition", "nature_mutation", "valeur_fonciere",
	"adresse_numero", "adresse_suffixe", "adresse_nom_voie", "adresse_code_voie", "code_postal",
	"code_commune", "nom_commune", "code_departement", "ancien_code_commune", "ancien_nom_commune",
	"id_parcelle", "ancien_id_parcelle", "numero_volume",
	"lot1_numero", "lot1_surface_carrez", "lot2_numero", "lot2_surface_carrez",
	"lot3_numero", "lot3_surface_carrez", "lot4_numero", "lot4_surface_carrez",
	"lot5_numero", "lot5_surface_carrez", "nombre_lots",
	"code_type_local", "type_local", "surface_reelle_bati", "nombre_pieces_principales",
	"code_nature_culture", "nature_culture", "code_nature_culture_speciale", "nature_culture_speciale",
	"surface_terrain", "longitude", "latitude",
}

// libellesCulture : le fichier brut ne donne que le code, geo-dvf ajoute le
// libellé. Les natures de culture spéciales, plus d'une centaine, gardent leur
// code faute de table.
var libellesCulture = map[string]string{
	"AB": "terrains a bâtir", "AG": "terrains d'agrément", "B": "bois",
	"BF": "futaies feuillues", "BM": "futaies mixtes", "BO": "oseraies",
	"BP": "peupleraies", "BR": "futaies résineuses", "BS": "taillis sous futaie",
	"BT": "taillis simples", "CA": "carrières", "CH": "chemin de fer", "E": "eaux",
	"J": "jardins", "L": "landes", "LB": "landes boisées", "P": "prés",
	"PA": "pâtures", "PC": "pacages", "PE": "prés d'embouche", "PH": "herbages",
	"PP": "prés plantés", "S": "sols", "T": "terres", "TP": "terres plantées",
	"VE": "vergers", "VI": "vignes",
}

// ArchiveProvisoire indique qu'une archive vient du fichier brut.
func ArchiveProvisoire(chemin string) bool { return fichierExiste(chemin + suffixeProvisoire) }

// publieSurGeoDVF demande à geo-dvf, sans rien télécharger, s'il a publié le
// millésime depuis. Dans le doute, la réponse est non : l'archive provisoire
// reste en place.
func publieSurGeoDVF(ctx context.Context, annee, dep string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, urlArchive(annee, dep), nil)
	if err != nil {
		return false
	}
	resp, err := clientDVF.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// telechargerDGFiP produit l'archive départementale d'un millésime à partir du
// fichier brut.
func telechargerDGFiP(ctx context.Context, annee, dep, chemin string) ResultatTelechargement {
	res := ResultatTelechargement{Dep: dep, Annee: annee}

	verrouDGFiP.Lock()
	defer verrouDGFiP.Unlock()

	sources, err := sourcesDGFiP(ctx, annee)
	if err != nil {
		res.Err = fmt.Errorf("%s %s : %w", dep, annee, err)
		return res
	}
	nettoyerZipsPerimes(annee, sources)
	if len(sources) == 0 {
		res.Absent = true
		return res
	}

	var zips []string
	var publie time.Time
	for _, s := range sources {
		z, err := zipDGFiP(ctx, s)
		if err != nil {
			res.Err = fmt.Errorf("%s %s : %w", dep, annee, err)
			return res
		}
		zips = append(zips, z)
		if s.Publication.After(publie) {
			publie = s.Publication
		}
	}

	lignes, octets, err := convertirDGFiP(zips, annee, dep, chemin)
	if err != nil {
		res.Err = fmt.Errorf("%s %s : %w", dep, annee, err)
		return res
	}
	// Le fichier brut ne couvre ni l'Alsace-Moselle (livre foncier) ni
	// Mayotte : aucune ligne n'est un millésime absent, pas une erreur.
	if lignes == 0 {
		res.Absent = true
		return res
	}
	if !publie.IsZero() {
		EcrirePublication(chemin+suffixePublication, publie)
	}
	if err := ecrireAtomique(chemin+suffixeProvisoire, []byte("DGFiP\n")); err != nil {
		res.Err = err
		return res
	}
	res.Octets, res.Provisoire = octets, true
	return res
}

// ---------------------------------------------------------------------------
// Trouver et récupérer les fichiers
// ---------------------------------------------------------------------------

// SourceDGFiP : un fichier publié. L'URL change à chaque publication (elle
// contient un horodatage), d'où le passage obligé par l'API du catalogue.
type SourceDGFiP struct {
	Nom         string
	URL         string
	Publication time.Time
}

type ressourceCatalogue struct {
	URL          string    `json:"url"`
	LastModified time.Time `json:"last_modified"`
}

func sourcesDGFiP(ctx context.Context, annee string) ([]SourceDGFiP, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiDGFiP, nil)
	if err != nil {
		return nil, err
	}
	resp, err := clientDVF.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalogue DGFiP : %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalogue DGFiP : statut %s", resp.Status)
	}

	var jeu struct {
		Resources []ressourceCatalogue `json:"resources"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jeu); err != nil {
		return nil, fmt.Errorf("catalogue DGFiP : %w", err)
	}
	return ChoisirSourcesDGFiP(jeu.Resources, annee), nil
}

// ChoisirSourcesDGFiP retient les fichiers d'une année. Une année complète
// prime sur ses semestres ; sinon on prend les semestres disponibles, dans
// l'ordre.
func ChoisirSourcesDGFiP(ressources []ressourceCatalogue, annee string) []SourceDGFiP {
	var annuel, semestres []SourceDGFiP
	for _, r := range ressources {
		nom := path.Base(r.URL)
		m := motifFichierDGFiP.FindStringSubmatch(nom)
		if m == nil || m[1] != annee {
			continue
		}
		s := SourceDGFiP{Nom: nom, URL: r.URL, Publication: r.LastModified}
		if m[2] == "" {
			annuel = append(annuel, s)
		} else {
			semestres = append(semestres, s)
		}
	}
	if len(annuel) > 0 {
		return annuel[:1]
	}
	sort.Slice(semestres, func(i, j int) bool { return semestres[i].Nom < semestres[j].Nom })
	return semestres
}

// zipDGFiP renvoie le chemin du fichier en cache, en le téléchargeant si la
// copie locale ne correspond pas à la publication en ligne. Le nom du fichier
// ne change pas d'une publication à l'autre : c'est la date notée à côté qui
// dit si la copie est à jour.
func zipDGFiP(ctx context.Context, s SourceDGFiP) (string, error) {
	chemin, err := cheminCache("dgfip", s.Nom)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(chemin); err == nil &&
		LirePublication(chemin+suffixePublication).Equal(s.Publication) {
		return chemin, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := clientDVF.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s : %w", s.Nom, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s : statut %s", s.Nom, resp.Status)
	}
	if _, err := ecrireFluxAtomique(chemin, resp.Body); err != nil {
		return "", err
	}
	EcrirePublication(chemin+suffixePublication, s.Publication)
	return chemin, nil
}

// ---------------------------------------------------------------------------
// Conversion au format geo-dvf
// ---------------------------------------------------------------------------

// convertirDGFiP parcourt les fichiers bruts d'un millésime et écrit les lignes
// d'un département dans une archive au format geo-dvf. Le fichier décompressé
// fait plusieurs centaines de Mo : on le lit en flux, ligne à ligne, et on
// compresse au fil de l'eau.
func convertirDGFiP(zips []string, annee, dep, chemin string) (int, int64, error) {
	lecteur, ecrivain := io.Pipe()
	compte := make(chan int, 1)

	go func() {
		gz := gzip.NewWriter(ecrivain)
		tampon := bufio.NewWriterSize(gz, tailleTampon)
		n, err := filtrerDGFiP(zips, annee, dep, tampon)
		if err == nil {
			err = tampon.Flush()
		}
		if err == nil {
			err = gz.Close()
		}
		compte <- n
		ecrivain.CloseWithError(err) // nil : fin de flux normale
	}()

	octets, err := ecrireFluxAtomique(chemin, lecteur)
	lecteur.Close() // débloque la goroutine si l'écriture a échoué en route
	lignes := <-compte
	if err != nil {
		return 0, 0, err
	}
	if lignes == 0 {
		os.Remove(chemin)
		return 0, 0, nil
	}
	return lignes, octets, nil
}

// filtrerDGFiP écrit l'en-tête geo-dvf puis les lignes converties.
func filtrerDGFiP(zips []string, annee, dep string, w io.Writer) (int, error) {
	if _, err := io.WriteString(w, strings.Join(enteteGeoDVF, ",")+"\n"); err != nil {
		return 0, err
	}
	conv := &convertisseur{annee: annee}
	total := 0
	for _, z := range zips {
		n, err := filtrerZip(z, dep, conv, w)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func filtrerZip(chemin, dep string, conv *convertisseur, w io.Writer) (int, error) {
	archive, err := zip.OpenReader(chemin)
	if err != nil {
		return 0, fmt.Errorf("ouverture de %s : %w", chemin, err)
	}
	defer archive.Close()

	total := 0
	for _, f := range archive.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".txt") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return total, fmt.Errorf("%s : %w", f.Name, err)
		}
		n, err := conv.filtrer(r, dep, w)
		r.Close()
		total += n
		if err != nil {
			return total, fmt.Errorf("%s : %w", f.Name, err)
		}
	}
	return total, nil
}

// convertisseur garde l'état qui survit d'une ligne à l'autre : la numérotation
// des mutations.
type convertisseur struct {
	annee     string
	col       map[string]int
	cleAvant  string
	mutations int
}

func (c *convertisseur) filtrer(r io.Reader, dep string, w io.Writer) (int, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, tailleTampon), tailleLigneMaxBrut)

	if !sc.Scan() {
		return 0, fmt.Errorf("fichier vide : %w", sc.Err())
	}
	c.col = map[string]int{}
	for i, nom := range strings.Split(strings.TrimPrefix(strings.TrimRight(sc.Text(), "\r"), "\ufeff"), separateurDGFiP) {
		c.col[strings.TrimSpace(nom)] = i
	}
	if _, ok := c.col["Code departement"]; !ok {
		return 0, fmt.Errorf("en-tête inattendu, colonne « Code departement » absente")
	}

	n := 0
	for sc.Scan() {
		champs := strings.Split(strings.TrimRight(sc.Text(), "\r"), separateurDGFiP)
		if c.champ(champs, "Code departement") != dep {
			continue
		}
		if _, err := io.WriteString(w, strings.Join(c.convertir(champs), ",")+"\n"); err != nil {
			return n, err
		}
		n++
	}
	return n, sc.Err()
}

func (c *convertisseur) champ(champs []string, nom string) string {
	i, ok := c.col[nom]
	if !ok || i >= len(champs) {
		return ""
	}
	// Une seule substitution règle deux problèmes. Dans un nombre, la virgule
	// décimale devient le point que strconv sait lire. Dans un libellé, elle
	// décalerait toutes les colonnes d'un fichier sans guillemets : geo-dvf la
	// remplace aussi par un point.
	return strings.ReplaceAll(strings.TrimSpace(champs[i]), ",", ".")
}

// convertir réécrit une ligne brute dans l'ordre des colonnes geo-dvf.
func (c *convertisseur) convertir(champs []string) []string {
	v := func(nom string) string { return c.champ(champs, nom) }

	dep := v("Code departement")
	insee := CodeInsee(dep, v("Code commune"))

	// Le fichier brut n'identifie pas les mutations : les lignes d'une même
	// mutation se suivent et partagent date, nature et prix. Une rupture sur
	// l'un d'eux ouvre une nouvelle mutation. La commune n'en fait pas partie :
	// une vente peut réunir des parcelles de deux communes voisines pour un
	// prix unique. Le préfixe « D » évite toute confusion avec les
	// identifiants d'Etalab.
	cle := strings.Join([]string{v("Date mutation"), v("Nature mutation"), v("Valeur fonciere")}, "|")
	if cle != c.cleAvant {
		c.mutations++
		c.cleAvant = cle
	}

	culture := v("Nature culture")
	speciale := v("Nature culture speciale")

	return []string{
		fmt.Sprintf("%s-D%d", c.annee, c.mutations),
		dateISO(v("Date mutation")),
		v("No disposition"),
		v("Nature mutation"),
		v("Valeur fonciere"),
		v("No voie"),
		v("B/T/Q"),
		strings.TrimSpace(v("Type de voie") + " " + v("Voie")),
		v("Code voie"),
		completerZeros(v("Code postal"), 5),
		insee,
		v("Commune"),
		dep,
		"", "", // ancienne commune : geo-dvf la déduit du code officiel géographique
		IDParcelle(insee, v("Prefixe de section"), v("Section"), v("No plan")),
		"",
		v("No Volume"),
		v("1er lot"), v("Surface Carrez du 1er lot"),
		v("2eme lot"), v("Surface Carrez du 2eme lot"),
		v("3eme lot"), v("Surface Carrez du 3eme lot"),
		v("4eme lot"), v("Surface Carrez du 4eme lot"),
		v("5eme lot"), v("Surface Carrez du 5eme lot"),
		v("Nombre de lots"),
		v("Code type local"),
		v("Type local"),
		v("Surface reelle bati"),
		v("Nombre pieces principales"),
		culture, libellesCulture[culture],
		speciale, speciale,
		v("Surface terrain"),
		"", "", // ni longitude ni latitude dans le fichier brut
	}
}

// CodeInsee recolle le département et le code commune. Le code INSEE fait
// toujours cinq caractères : trois pour la commune en métropole et en Corse
// (01 + 244, 2A + 004), deux outre-mer où le département en prend trois
// (971 + 10).
func CodeInsee(dep, commune string) string {
	if dep == "" || commune == "" {
		return ""
	}
	return dep + completerZeros(commune, 5-len(dep))
}

// IDParcelle reconstitue l'identifiant cadastral sur quatorze caractères :
// commune (5), préfixe de section (3, « 000 » hors communes fusionnées),
// section (2), numéro de plan (4).
func IDParcelle(insee, prefixe, section, plan string) string {
	if insee == "" || section == "" || plan == "" {
		return ""
	}
	if prefixe == "" {
		prefixe = "000"
	}
	return insee + completerZeros(prefixe, 3) + completerZeros(section, 2) + completerZeros(plan, 4)
}

func completerZeros(s string, n int) string {
	if s == "" || len(s) >= n {
		return s
	}
	return strings.Repeat("0", n-len(s)) + s
}

// dateISO passe de jj/mm/aaaa à aaaa-mm-jj. Une date illisible est laissée
// telle quelle : le lecteur l'écartera plutôt que d'inventer.
func dateISO(s string) string {
	t, err := time.Parse("02/01/2006", s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02")
}

// nettoyerZipsPerimes supprime du cache les fichiers bruts d'une année qui ne
// figurent plus au catalogue — un semestre remplacé par l'année complète.
func nettoyerZipsPerimes(annee string, gardes []SourceDGFiP) {
	base, err := cheminCache("dgfip", "x")
	if err != nil {
		return
	}
	entrees, err := os.ReadDir(filepath.Dir(base))
	if err != nil {
		return
	}
	garde := map[string]bool{}
	for _, s := range gardes {
		garde[s.Nom] = true
	}
	for _, e := range entrees {
		m := motifFichierDGFiP.FindStringSubmatch(e.Name())
		if m == nil || m[1] != annee || garde[e.Name()] {
			continue
		}
		chemin := filepath.Join(filepath.Dir(base), e.Name())
		os.Remove(chemin)
		os.Remove(chemin + suffixePublication)
	}
}

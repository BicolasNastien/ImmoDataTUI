package main

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Récupération des données DVF à la demande
// ---------------------------------------------------------------------------
//
// Le jeu « DVF géolocalisées » est publié par département et par millésime :
//
//	https://files.data.gouv.fr/geo-dvf/latest/csv/<année>/departements/<dd>.csv.gz
//
// Le chemin « latest » suit les publications semestrielles, et une publication
// peut ajouter des transactions dans les millésimes passés : un fichier
// téléchargé n'est jamais définitif, d'où la péremption du cache ci-dessous.

const (
	urlDVF         = "https://files.data.gouv.fr/geo-dvf/latest/csv/%s/departements/%s.csv.gz"
	nbAnnees       = 6
	peremption     = 90 * 24 * time.Hour // deux publications par an : trois mois est un bon compromis
	parallelisme   = 3                   // requêtes simultanées vers un serveur public : rester poli
	tailleTampon   = 64 * 1024
	permDossier    = 0o755
	permFichier    = 0o644
	entetesMinimal = "id_mutation"
)

// clientDVF : délai généreux (les archives font quelques mégaoctets) mais fini.
var clientDVF = &http.Client{Timeout: 15 * time.Minute}

// jetons limite le nombre de téléchargements simultanés. Un channel tamponné
// suffit à faire un sémaphore ; la valeur transportée n'a aucune importance,
// d'où la structure vide.
var jetons = make(chan struct{}, parallelisme)

// AnneesCandidates renvoie les millésimes à tenter, du plus récent au plus
// ancien. Le millésime de l'année en cours n'existe qu'à partir de la
// publication d'avril : son absence est normale, pas une erreur.
func AnneesCandidates() []string {
	courante := time.Now().Year()
	var out []string
	for i := 0; i < nbAnnees; i++ {
		out = append(out, strconv.Itoa(courante-i))
	}
	return out
}

// CheminArchive renvoie l'emplacement de cache d'une archive départementale.
func CheminArchive(annee, dep string) (string, error) {
	return cheminCache("dvf", annee, dep+".csv.gz")
}

// ArchiveFraiche indique si l'archive est présente et pas encore périmée.
func ArchiveFraiche(annee, dep string) bool {
	chemin, err := CheminArchive(annee, dep)
	if err != nil {
		return false
	}
	info, err := os.Stat(chemin)
	return err == nil && info.Size() > 0 && time.Since(info.ModTime()) < peremption
}

// ResultatTelechargement rend compte d'un millésime.
type ResultatTelechargement struct {
	Dep    string
	Annee  string
	Octets int64
	DejaLa bool
	Absent bool // millésime non publié : ce n'est pas une erreur
	// Provisoire : tiré du fichier brut de la DGFiP faute de geo-dvf, donc
	// sans coordonnées. Remplacé dès que geo-dvf publie le millésime.
	Provisoire bool
	Err        error
}

// TelechargerArchive récupère un millésime départemental dans le cache.
// Le contexte permet d'abandonner proprement sur une touche d'échappement.
func TelechargerArchive(ctx context.Context, annee, dep string, forcer bool) ResultatTelechargement {
	res := ResultatTelechargement{Dep: dep, Annee: annee}

	chemin, err := CheminArchive(annee, dep)
	if err != nil {
		res.Err = err
		return res
	}
	// Une archive provisoire reste valable tant que geo-dvf n'a rien de mieux.
	// Une requête HEAD suffit à le savoir.
	provisoire := ArchiveProvisoire(chemin)
	if !forcer && ArchiveFraiche(annee, dep) && !(provisoire && publieSurGeoDVF(ctx, annee, dep)) {
		info, _ := os.Stat(chemin)
		res.DejaLa, res.Octets, res.Provisoire = true, info.Size(), provisoire
		if provisoire {
			return res // sa date vient du catalogue DGFiP, pas de geo-dvf
		}
		completerPublication(ctx, urlArchive(annee, dep), chemin, info.ModTime())
		return res
	}

	select {
	case jetons <- struct{}{}:
		defer func() { <-jetons }()
	case <-ctx.Done():
		res.Err = ctx.Err()
		return res
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlArchive(annee, dep), nil)
	if err != nil {
		res.Err = err
		return res
	}

	resp, err := clientDVF.Do(req)
	if err != nil {
		res.Err = fmt.Errorf("%s %s : %w", dep, annee, err)
		return res
	}
	defer resp.Body.Close()

	// Un 404 n'est pas une erreur pour net/http : sans cette vérification on
	// écrirait une page HTML d'erreur dans le cache et on la relirait ensuite
	// comme si c'était des données. Un millésime absent de geo-dvf peut
	// pourtant exister dans le fichier brut de la DGFiP, publié plus tôt.
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return telechargerDGFiP(ctx, annee, dep, chemin)
	}
	if resp.StatusCode != http.StatusOK {
		res.Err = fmt.Errorf("%s %s : statut %s", dep, annee, resp.Status)
		return res
	}

	n, err := ecrireFluxAtomique(chemin, resp.Body)
	if err != nil {
		res.Err = err
		return res
	}
	res.Octets = n
	os.Remove(chemin + suffixeProvisoire) // geo-dvf remplace le fichier brut
	if publie, err := http.ParseTime(resp.Header.Get("Last-Modified")); err == nil {
		EcrirePublication(chemin+suffixePublication, publie)
	}
	return res
}

func urlArchive(annee, dep string) string { return fmt.Sprintf(urlDVF, annee, dep) }

// ---------------------------------------------------------------------------
// Date de publication
// ---------------------------------------------------------------------------
//
// Le contenu d'un fichier ne dit pas de quelle publication il provient : seul
// l'en-tête Last-Modified du serveur le sait. On le note à côté de l'archive au
// moment du téléchargement, puis on le recopie dans chaque extrait communal :
// la date voyage avec les données qu'elle décrit. La date de modification du
// fichier local, elle, est prise par la péremption du cache — et ne dirait que
// le jour du téléchargement.

const (
	suffixePublication = ".publication"
	fichierPublication = ".publication" // dans un dossier millésime : ignoré par le lecteur, qui ne lit que les .csv
)

// EcrirePublication note une date de publication dans un fichier texte.
func EcrirePublication(chemin string, t time.Time) error {
	return ecrireAtomique(chemin, []byte(t.UTC().Format(time.RFC3339)+"\n"))
}

// LirePublication relit une date notée par EcrirePublication. Le zéro signifie
// « inconnue » : une collection déposée à la main n'en a pas.
func LirePublication(chemin string) time.Time {
	brut, err := os.ReadFile(chemin)
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(brut)))
	if err != nil {
		return time.Time{}
	}
	return t
}

// completerPublication rattrape la date d'une archive téléchargée avant qu'on
// la note. Une requête HEAD donne la date de la version en ligne ; elle n'est
// celle de l'archive en cache que si l'archive est plus récente qu'elle — sinon
// une publication est sortie depuis, et mieux vaut ne rien dire que mentir.
func completerPublication(ctx context.Context, adresse, chemin string, telecharge time.Time) {
	if !LirePublication(chemin + suffixePublication).IsZero() {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, adresse, nil)
	if err != nil {
		return
	}
	resp, err := clientDVF.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
	publie, err := http.ParseTime(resp.Header.Get("Last-Modified"))
	if err == nil && resp.StatusCode == http.StatusOK && !publie.After(telecharge) {
		EcrirePublication(chemin+suffixePublication, publie)
	}
}

// DepartementsTelecharges liste les départements dont au moins un millésime est
// en cache. C'est ce qui permet de teinter la carte selon ce qu'on possède.
func DepartementsTelecharges() map[string]bool {
	out := map[string]bool{}
	base, err := cheminCache("dvf", "x")
	if err != nil {
		return out
	}
	racine := filepath.Dir(base)

	annees, err := os.ReadDir(racine)
	if err != nil {
		return out
	}
	for _, a := range annees {
		if !a.IsDir() {
			continue
		}
		fichiers, err := os.ReadDir(filepath.Join(racine, a.Name()))
		if err != nil {
			continue
		}
		for _, f := range fichiers {
			if nom := strings.TrimSuffix(f.Name(), ".csv.gz"); nom != f.Name() {
				out[nom] = true
			}
		}
	}
	return out
}

// AnneesEnCache renvoie les millésimes présents pour un département, du plus
// récent au plus ancien.
func AnneesEnCache(dep string) []string {
	var out []string
	for _, a := range AnneesCandidates() {
		chemin, err := CheminArchive(a, dep)
		if err != nil {
			continue
		}
		if info, err := os.Stat(chemin); err == nil && info.Size() > 0 {
			out = append(out, a)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

// ---------------------------------------------------------------------------
// Extraction d'une commune
// ---------------------------------------------------------------------------

// ExtraireCommune découpe les archives départementales en cache pour n'en
// garder que les lignes d'une commune, écrites dans l'arborescence que le
// programme sait déjà lire :
//
//	<racine>/<Nom-Commune>/<année>/<code INSEE>.csv
//
// Faire converger les chemins de données plutôt que les multiplier : un fichier
// téléchargé et un fichier déposé à la main deviennent indiscernables, et la
// collection existante continue de fonctionner sans migration.
func ExtraireCommune(racine, dep, codeInsee, nom string) ([]string, error) {
	dossier := filepath.Join(racine, NormaliserNom(nom))
	var annees []string

	for _, annee := range AnneesEnCache(dep) {
		archive, err := CheminArchive(annee, dep)
		if err != nil {
			return nil, err
		}
		cible := filepath.Join(dossier, annee, codeInsee+".csv")

		// Déjà extrait et plus récent que l'archive : rien à refaire.
		if infoCible, err := os.Stat(cible); err == nil {
			if infoArch, err := os.Stat(archive); err == nil && infoCible.ModTime().After(infoArch.ModTime()) {
				annees = append(annees, annee)
				recopierPublication(archive, filepath.Dir(cible))
				continue
			}
		}

		lignes, err := extraireVersFichier(archive, codeInsee, cible)
		if err != nil {
			return nil, err
		}
		if lignes > 0 {
			annees = append(annees, annee)
			recopierPublication(archive, filepath.Dir(cible))
		} else {
			os.Remove(cible)
			os.Remove(filepath.Dir(cible))
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(annees)))
	return annees, nil
}

// recopierPublication reporte la date de l'archive, et sa marque provisoire
// éventuelle, dans le dossier d'un extrait.
// Sans date connue, on efface l'ancienne : une date périmée serait pire
// qu'aucune.
func recopierPublication(archive, dossier string) {
	note := filepath.Join(dossier, fichierPublication)
	if publie := LirePublication(archive + suffixePublication); !publie.IsZero() {
		EcrirePublication(note, publie)
	} else {
		os.Remove(note)
	}

	// La marque provisoire suit le même chemin : c'est l'extrait que lit
	// l'écran de statistiques, pas l'archive.
	marque := filepath.Join(dossier, fichierProvisoire)
	if ArchiveProvisoire(archive) {
		ecrireAtomique(marque, []byte("DGFiP\n"))
	} else {
		os.Remove(marque)
	}
}

// extraireVersFichier filtre une archive sur le code commune. Le fichier est
// recopié tel quel, séparateur d'origine compris : le lecteur CSV le détecte.
func extraireVersFichier(archive, codeInsee, cible string) (int, error) {
	f, err := os.Open(archive)
	if err != nil {
		return 0, fmt.Errorf("ouverture de %s : %w", archive, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, fmt.Errorf("décompression de %s : %w", archive, err)
	}
	defer gz.Close()

	brut, err := io.ReadAll(gz)
	if err != nil {
		return 0, fmt.Errorf("lecture de %s : %w", archive, err)
	}

	lignes := strings.Split(string(brut), "\n")
	if len(lignes) == 0 || !strings.HasPrefix(lignes[0], entetesMinimal) {
		return 0, fmt.Errorf("%s : en-tête inattendu", archive)
	}

	sep := detecterSeparateur(lignes[0])
	colonne := indexColonne(lignes[0], sep, "code_commune")
	if colonne < 0 {
		return 0, fmt.Errorf("%s : colonne code_commune absente", archive)
	}

	var b strings.Builder
	b.WriteString(lignes[0])
	b.WriteByte('\n')
	n := 0
	for _, ligne := range lignes[1:] {
		if ligne == "" {
			continue
		}
		champs := strings.Split(ligne, string(sep))
		if colonne < len(champs) && champs[colonne] == codeInsee {
			b.WriteString(ligne)
			b.WriteByte('\n')
			n++
		}
	}
	if n == 0 {
		return 0, nil
	}

	if err := os.MkdirAll(filepath.Dir(cible), permDossier); err != nil {
		return 0, fmt.Errorf("création de %s : %w", filepath.Dir(cible), err)
	}
	if err := ecrireAtomique(cible, []byte(b.String())); err != nil {
		return 0, err
	}
	return n, nil
}

func indexColonne(entete string, sep rune, nom string) int {
	for i, c := range strings.Split(entete, string(sep)) {
		if strings.TrimSpace(strings.Trim(c, "\ufeff\r\n")) == nom {
			return i
		}
	}
	return -1
}

// detecterSeparateur tranche entre virgule et point-virgule. Les fichiers
// départementaux publiés utilisent la virgule, les extraits par commune
// diffusés ailleurs le point-virgule : autant lire les deux.
func detecterSeparateur(entete string) rune {
	if strings.Count(entete, ";") > strings.Count(entete, ",") {
		return ';'
	}
	return ','
}

// ---------------------------------------------------------------------------
// Écriture sur disque
// ---------------------------------------------------------------------------

// ecrireFluxAtomique copie un flux dans un fichier temporaire du même
// répertoire, puis le renomme. Le renommage est atomique sur un même système de
// fichiers : une interruption en cours de route ne laisse jamais un fichier
// tronqué que le programme relirait comme valide.
func ecrireFluxAtomique(chemin string, source io.Reader) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(chemin), permDossier); err != nil {
		return 0, fmt.Errorf("création de %s : %w", filepath.Dir(chemin), err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(chemin), ".partiel-*")
	if err != nil {
		return 0, fmt.Errorf("fichier temporaire : %w", err)
	}
	defer os.Remove(tmp.Name()) // sans effet si le renommage a réussi

	n, err := io.CopyBuffer(tmp, source, make([]byte, tailleTampon))
	if err != nil {
		tmp.Close()
		return 0, fmt.Errorf("écriture de %s : %w", chemin, err)
	}
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("fermeture de %s : %w", tmp.Name(), err)
	}
	if err := os.Chmod(tmp.Name(), permFichier); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp.Name(), chemin); err != nil {
		return 0, fmt.Errorf("renommage vers %s : %w", chemin, err)
	}
	return n, nil
}

func ecrireAtomique(chemin string, contenu []byte) error {
	_, err := ecrireFluxAtomique(chemin, strings.NewReader(string(contenu)))
	return err
}

func lireTout(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

// verifierCSV n'est utilisé que par les tests : il valide qu'un extrait se
// relit bien avec le séparateur détecté.
func verifierCSV(chemin string) (int, error) {
	f, err := os.Open(chemin)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	tete := make([]byte, 4096)
	n, _ := f.Read(tete)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	r := csv.NewReader(f)
	r.Comma = detecterSeparateur(string(tete[:n]))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	enregistrements, err := r.ReadAll()
	if err != nil {
		return 0, err
	}
	return len(enregistrements), nil
}

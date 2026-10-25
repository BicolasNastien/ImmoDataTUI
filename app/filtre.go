package main

// Filtre de la liste des communes.
//
// Un département compte jusqu'à huit cents communes : les faire défiler une à
// une pour atteindre la bonne est pénible. « / » ouvre un filtre, à la manière
// de less ou de vim : chaque lettre tapée réduit la liste, ↵ ouvre la commune
// sélectionnée. La carte, elle, continue de montrer tout le département — la
// sélection y reste visible, et c'est souvent ce qu'on cherche : où est-elle ?

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// correspondFiltre compare sans accents, sans casse et sans séparateurs :
// « st jean » ne trouverait rien, mais « saint-jean », « saint jean » et
// « SAINT JEAN » trouvent tous Saint-Jean-de-Luz. Un code INSEE ou son début
// marche aussi, pour qui le connaît.
func correspondFiltre(z Zone, filtre string) bool {
	f := clefLibelle(filtre)
	if f == "" {
		return true
	}
	return strings.Contains(clefLibelle(z.Nom), f) || strings.HasPrefix(strings.ToLower(z.Code), f)
}

// visibles renvoie les indices des zones que la liste affiche, dans l'ordre
// d'affichage. Les noms qui commencent par le filtre passent devant ceux qui
// le contiennent seulement : en tapant « bou », on veut Boucau avant
// Saint-Martin-de-Boulogne.
func (e etatCarte) visibles() []int {
	out := make([]int, 0, len(e.zones))
	if clefLibelle(e.filtre) == "" {
		for i := range e.zones {
			out = append(out, i)
		}
		return out
	}
	f := clefLibelle(e.filtre)
	var ensuite []int
	for i, z := range e.zones {
		switch {
		case !correspondFiltre(z, e.filtre):
		case strings.HasPrefix(clefLibelle(z.Nom), f):
			out = append(out, i)
		default:
			ensuite = append(ensuite, i)
		}
	}
	return append(out, ensuite...)
}

// position renvoie le rang de la sélection dans la liste affichée, -1 si elle
// n'y figure pas (le survol de la carte peut sélectionner une commune filtrée).
func position(vis []int, sel int) int {
	for p, i := range vis {
		if i == sel {
			return p
		}
	}
	return -1
}

// filtrer applique un nouveau filtre : la sélection passe à la meilleure
// correspondance, ou à rien du tout. Garder l'ancienne sélection, invisible,
// ferait ouvrir à ↵ une commune que l'utilisateur ne voit pas.
func (e *etatCarte) filtrer(filtre string) {
	e.filtre = filtre
	e.haut = 0
	if vis := e.visibles(); len(vis) > 0 {
		e.sel = vis[0]
	} else {
		e.sel = -1
	}
}

// filtreActif : la liste est réduite, ou en cours de saisie.
func (e etatCarte) filtreActif() bool { return e.saisitFiltre || e.filtre != "" }

func (m model) ouvrirFiltre() (tea.Model, tea.Cmd) {
	m.carte.saisitFiltre = true
	return m, nil
}

// effacerFiltre rend la liste complète en gardant la commune sélectionnée.
func (m model) effacerFiltre() (tea.Model, tea.Cmd) {
	sel := m.carte.sel
	m.carte.filtrer("")
	m.carte.saisitFiltre = false
	if sel >= 0 {
		m.carte.sel = sel
	}
	if m.carte.carte != nil {
		m.carte.suivre(m.carte.carte.H)
	}
	return m, nil
}

// toucheFiltre : pendant la saisie, le clavier appartient au filtre. Sans
// cela, taper « q » dans « Quimper » quitterait le programme, et « a » dans
// « Anglet » ouvrirait les annonces.
func (m model) toucheFiltre(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		return m.effacerFiltre()
	case tea.KeyEnter:
		m.carte.saisitFiltre = false
		if _, ok := m.carte.selection(); !ok {
			return m, nil
		}
		return m.ouvrir()
	case tea.KeyUp:
		return m.deplacer(-1), nil
	case tea.KeyDown:
		return m.deplacer(1), nil
	case tea.KeyBackspace:
		r := []rune(m.carte.filtre)
		if len(r) == 0 {
			// Effacer au-delà du début referme la saisie, comme dans less.
			return m.effacerFiltre()
		}
		m.carte.filtrer(string(r[:len(r)-1]))
		return m, nil
	case tea.KeySpace:
		m.carte.filtrer(m.carte.filtre + " ")
		return m, nil
	case tea.KeyRunes:
		m.carte.filtrer(m.carte.filtre + string(msg.Runes))
		return m, nil
	}
	return m, nil
}

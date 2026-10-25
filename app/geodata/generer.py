#!/usr/bin/env python3
"""Génère les contours embarqués (régions et départements) d'ImmoDataTUI.

Sources :
  - métropole : github.com/gregoiredavid/france-geojson (regions/departements)
  - DROM      : geo.api.gouv.fr, contours des communes fusionnés par département
                (l'API ne sert pas de contour pour les régions/départements)

Les contours sont simplifiés (Douglas-Peucker) et arrondis : la carte est rendue
sur une grille de quelques centaines de pixels, une précision au kilomètre est
largement suffisante. À relancer seulement si le découpage administratif change.

    python3 generer.py            # écrit regions.json et departements.json
"""

import json
import math
import os
import urllib.request

ICI = os.path.dirname(os.path.abspath(__file__))

SRC_REGIONS = "https://raw.githubusercontent.com/gregoiredavid/france-geojson/master/regions.geojson"
SRC_DEPS = "https://raw.githubusercontent.com/gregoiredavid/france-geojson/master/departements.geojson"
API = "https://geo.api.gouv.fr"

# Les DROM, absents de france-geojson. région -> (nom, département)
DROM = {
    "01": ("Guadeloupe", "971"),
    "02": ("Martinique", "972"),
    "03": ("Guyane", "973"),
    "04": ("La Réunion", "974"),
    "06": ("Mayotte", "976"),
}

TOLERANCE = 0.008   # degrés (~800 m) pour la simplification
AIRE_MINI = 6.0     # km² : en dessous, l'îlot est invisible à l'écran
DECIMALES = 4


def telecharger(url):
    req = urllib.request.Request(url, headers={"User-Agent": "immodata/geodata"})
    with urllib.request.urlopen(req, timeout=120) as r:
        return json.load(r)


def dp(points, tol):
    """Douglas-Peucker sur une liste de (lon, lat)."""
    if len(points) < 3:
        return points
    ax, ay = points[0]
    bx, by = points[-1]
    dx, dy = bx - ax, by - ay
    norme = math.hypot(dx, dy)
    imax, dmax = 0, 0.0
    for i in range(1, len(points) - 1):
        px, py = points[i]
        if norme == 0:
            d = math.hypot(px - ax, py - ay)
        else:
            d = abs(dy * px - dx * py + bx * ay - by * ax) / norme
        if d > dmax:
            imax, dmax = i, d
    if dmax <= tol:
        return [points[0], points[-1]]
    return dp(points[: imax + 1], tol)[:-1] + dp(points[imax:], tol)


def aire_km2(ring):
    """Aire approchée d'un anneau en km² (shoelace corrigée de la latitude)."""
    if len(ring) < 3:
        return 0.0
    lat = sum(p[1] for p in ring) / len(ring)
    k = math.cos(math.radians(lat))
    s = 0.0
    for i in range(len(ring)):
        x1, y1 = ring[i]
        x2, y2 = ring[(i + 1) % len(ring)]
        s += (x1 * k) * y2 - (x2 * k) * y1
    return abs(s) / 2 * 111.32 * 111.32


def anneaux(geom):
    """Aplatit une géométrie GeoJSON en liste d'anneaux extérieurs.

    Les trous sont ignorés : le remplissage pair-impair du rendu les gérerait,
    mais aucun département n'en a d'assez grand pour se voir en terminal.
    """
    t = geom["type"]
    if t == "Polygon":
        return [geom["coordinates"][0]]
    if t == "MultiPolygon":
        return [poly[0] for poly in geom["coordinates"]]
    raise ValueError("géométrie inattendue : " + t)


def nettoyer(rings):
    """Simplifie, filtre les îlots minuscules, arrondit. Garde toujours le plus gros."""
    out = []
    for r in rings:
        pts = [(float(x), float(y)) for x, y in r]
        pts = dp(pts, TOLERANCE)
        if len(pts) < 4:
            continue
        out.append((aire_km2(pts), pts))
    if not out:
        return []
    out.sort(key=lambda a: -a[0])
    gardes = [out[0]] + [x for x in out[1:] if x[0] >= AIRE_MINI]
    plats = []
    for _, pts in gardes:
        plat = []
        for x, y in pts:
            plat.append(round(x, DECIMALES))
            plat.append(round(y, DECIMALES))
        plats.append(plat)
    return plats


def ecrire(nom, zones):
    chemin = os.path.join(ICI, nom)
    with open(chemin, "w", encoding="utf-8") as f:
        json.dump(zones, f, ensure_ascii=False, separators=(",", ":"))
    ko = os.path.getsize(chemin) / 1024
    pts = sum(len(r) // 2 for z in zones for r in z["c"])
    print(f"{nom}: {len(zones)} zones, {pts} points, {ko:.0f} Ko")


def contours_drom(dep):
    """Contour d'un département d'outre-mer : union des contours de ses communes."""
    url = f"{API}/departements/{dep}/communes?fields=contour&format=json&geometry=contour"
    rings = []
    for c in telecharger(url):
        if c.get("contour"):
            rings += anneaux(c["contour"])
    return rings


def main():
    regions, deps = [], []

    for feat in telecharger(SRC_REGIONS)["features"]:
        p = feat["properties"]
        regions.append({"code": p["code"], "nom": p["nom"], "c": nettoyer(anneaux(feat["geometry"]))})

    for feat in telecharger(SRC_DEPS)["features"]:
        p = feat["properties"]
        deps.append({"code": p["code"], "nom": p["nom"], "c": nettoyer(anneaux(feat["geometry"]))})

    # Rattachement département -> région (l'API donne le lien, pas la géométrie).
    rattachement = {d["code"]: d["codeRegion"] for d in telecharger(f"{API}/departements?fields=code,codeRegion")}

    for codeReg, (nom, codeDep) in DROM.items():
        rings = nettoyer(contours_drom(codeDep))
        regions.append({"code": codeReg, "nom": nom, "c": rings})
        deps.append({"code": codeDep, "nom": nom, "c": rings})

    for d in deps:
        d["region"] = rattachement.get(d["code"], "")

    regions.sort(key=lambda z: z["code"])
    deps.sort(key=lambda z: z["code"])
    ecrire("regions.json", regions)
    ecrire("departements.json", deps)


if __name__ == "__main__":
    main()

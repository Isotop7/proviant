# UI-Refresh-Plan (abgelöst)

> ** superseded — nicht mehr ausführen.**

Dieser Plan vom 2026-10-04 ist durch die Migration
*„UI-Migration zum `design-system/`"* abgelöst. Er ist hier nur noch
der Vollständigkeit halber verlinkt.

## Warum er abgelöst wurde

Der Plan beschrieb eine/token-getriebene Erneuerung von `main.scss` und
`login.scss`, als wäre `main.scss` eine einzige Datei gewesen. Inzwischen
liegt der Stylesheet-Code in 16 Partialen, die App trägt bereits eine
funktionierende Dark-Theme-Mechanik, und die Zeilennummern im Plan sind
längst überholt.

Drei Dinge, die der Plan richtig getroffen hat und die weiterhin gelten:

- Die Token-Werte gehören in **eine** Quelle, nicht in zwei.
- Buttons, Badges und Formulare gehören auf **Bootstrap-Klassen**, nicht auf
  eigene.
- Der Dark Theme bleibt auf `data-bs-theme` + `theme.js` — die
  `tokens.dark.css` des Specs war widersprüchlich.

## Was stattdessen gilt

| Dokument | Inhalt |
|---|---|
| `design-system/_tokens.scss` | die Token-Werte (kanonische Quelle) |
| `design-system/DESIGN.md` | die normativen Regeln des Design-Systems |
| `design-system/deviations.md` | jede Stelle, an der davon abgewichen wird |
| `design-system/nav.md` | die Menü-Struktur |
| `design-system/components.manifest.json` | der maschinenlesbare Komponenten-Vertrag |
| `design-system/screens/` | die Anordnung pro Bildschirm |

Arbeitsstand und Reihenfolge: siehe den Migrationsplan. Die Prüfungen laufen
über `task design-tokens` und `task design-check`.

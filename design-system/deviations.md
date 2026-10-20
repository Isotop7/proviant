# Abweichungen vom Design-System

> Jede Stelle, an der das Design-System **nicht** umgesetzt wird, steht hier.
> Kein Eintrag ohne Grund. Geprüft von `task design-check`.

Die `DESIGN.md` ist die normative Schicht: Tokens, Farbe, Typografie, Spacing,
Radius, Elevation, Motion, A11y und die Bootstrap-Klassen-Pflicht gelten für
alle Bildschirme. Die `screens/` sind die normative Schicht für die
*Anordnung* einer Seite. Wo die beiden sich widersprechen, entscheidet die
jeweilige Schicht — diese Liste hält fest, was daraus abweicht.

**Regel für neue Einträge:** eine Abweichung braucht eine Begründung, die
unabhängig von der aktuellen Implementierung gültig ist, und — wenn sie
sichtbar ist — einen messbaren Beleg (Kontrastwert, Route, Endpoint).

| # | Abweichung | Grund |
|---|---|---|
| 1 | `.field--code` wird nicht gebaut | Es gibt keinen Code-Flow: Verify, Passwort-Reset und Einladung nutzen alle opake URL-Tokens. Ein 6-stelliger Code-Input hätte keinen Verbraucher. |
| 2 | Keine Produkt-Detailseite, Erstellen bleibt im Modal | Eine Migration baut keine Routen. Für die Detailseite existiert kein schreibgeschütztes View-Model; sie ist als Feature-PR vorgesehen, nicht als Teil dieser Migration. |
| 3 | `.kpi__delta` und `.kpi__sparkline` werden nicht gebaut | Die Stats-API liefert pro Kachel nur Momentanwerte. Die einzige Zeitreihe (`expiryTrend`) ist eine **Vorausprojektion** der nächsten zwölf Monate, keine Historie. Eine Sparkline hätte keine Datengrundlage. |
| 4 | `--status-neutral` kommt hinzu | Die App kennt fünf Zustände, das Quadrupel des Specs nur vier. Der fünfte ("kein Ablaufdatum") mappt auf das im Manifest vorgesehene `--badge--neutral`. |
| 5 | `.badge--lowstock` und `.badge--opened` sind App-Erweiterungen | Die fünf Badge-Modifier des Specs decken ausschließlich Ablaufzustände ab. "Niedriger Bestand" und "geöffnet" sind keine Ablaufzustände. |
| 6 | Die Produktliste bleibt ein CSS-Grid, kein Bootstrap `.table` | Das Spec-Sheet zeigt dieselbe Grid-Struktur. Eine echte `<table>` mit sieben Spalten bricht das 360-px-Layout; das Grid ist responsiv kollabierbar. |
| 7 | Button-Beschriftungen bleiben in Sentence-Case | Die Uppercase-Regel kompensiert einen Kontrastmangel des alten Akzents. Mit `--accent` #418069 (4.65:1) ist die Kompensation nicht mehr nötig — und im deutschen Katalog werden Komposita durch Großschreibung unlesbar. Großbuchstaben bleiben dem `.eyebrow`-Label vorbehalten. |
| 8 | Neue Textvarianten `--accent-text` und `--status-*-text` | Die Basisfarben des Specs fallen als Text unter die WCAG-Schwelle (soon 2.18:1, critical 3.33:1, fresh 3.97:1 auf ihrem 18-%-Tint). Die Basisfarbe trägt Icons, Punkte, Balken und Rahmen; Text nimmt die dunklere Variante. |
| 9 | `--status-soon` ist von `#C2A24A` auf `#A58529` abgedunkelt | Der Spec-Wert erreicht auf dem Crème-Hintergrund nur 2.29:1. Ein Statuspunkt ist eine Zustandsanzeige und braucht 3:1. Farbton (89°) und Chroma (0.113) bleiben unverändert. |
| 10 | `--meta` ist von `#8A8A8A` auf `#6F6F6F` abgedunkelt | Der Spec-Wert erreicht auf `--bg` nur 3.23:1. `--meta` trägt Zeitstempel und Captions, also Text. |
| 11 | Der blaue Sekundärton entfällt | Das Spec verbietet eine zweite Akzentfarbe ("proviant ist Salwei + Anthrazit"). `--brand-slate`, `--brand-slate-subtle`, `$secondary` und `$link-color` werden entfernt; Links tragen `--accent-text`. |
| 12 | Die `pv-*`-Klassen bleiben (23 distinkte) | Seiten-spezifische Layout-Klassen ohne Gegenstück im Spec. Der Namensraum trennt sie vom Design-System-Vokabular; jede Familie ist hier benannt. |
| 13 | `.icon-chip`, `.qty-stepper`, `.pack-size`, `.filter-pill`, `.hero-*` bleiben | App-spezifische Bausteine ohne Spec-Komponente. Sie werden auf die Spec-Tokens umgestellt, nicht umbenannt. |
| 14 | Bootstrap-`.dropdown-menu` wird weiter genutzt | Das Manifest verzichtete darauf; die Benachrichtigungsliste und das Mobile-„Mehr"-Menü brauchen es aber. Sie sind die einzigen vier Stellen. |
| 15 | `.text-body*` aus Bootstrap 5.3 trägt die Textebene | Das Spec definiert keine Text-Utilities. Die Bootstrap-Originale sind der native Gegenpart und vermeiden eine eigene Hierarchie. |
| 16 | Die Auth-Flow-Seiten behalten das einspaltige Layout | `acceptInvite`, `verifyEmail`, `onboarding` und `unsubscribe` sind Flow-Seiten, nicht die Anmeldefläche. Das geteilte Brand-Panel gilt für die Anmeldung. |
| 17 | `--fg-inverse`, `--card-min`, `--text-disabled` bleiben | App-eigene Slots ohne Spec-Gegenstück. |
| 18 | `.modal--demo` bleibt Spec-Sheet-only | Die App nutzt echte Bootstrap-Modals. Die Klasse existiert nur, damit die Sheets ein sichtbares Modal zeigen. |
| 19 | Das Kategorien-Diagramm nutzt `--info` als sechste Farbe | Das Design-System kennt keine kategoriale Palette: das Diagramm leiht sich die Statussprache. Vier Statusfarben plus Akzent ergeben fünf unterscheidbare Farben, die Haushalte mit mehr Kategorien brauchen eine sechste. Die Spec erlaubt sie für Diagramme nicht ausdrücklich. |
| 20 | Das Badge `opened` nutzt die Neutralfamilie statt Violett | Der Spec verbietet Violett, und "geöffnet" ist kein Ablaufzustand. Grau ist die neutrale Aussage; die dringenden Zustände bleiben farbig. |
| 21 | Neue Slots `--status-*-on` (Tinte auf voller Statusfläche) | Eine gefüllte Fläche braucht eine eigene Tinte: im hellen Theme Weiß, im dunklen Theme Tinte auf dem hellen Statuswert. Ohne sie trägt die Kachel in einem der beiden Themes unlesbaren Text. |
| 22 | `.modal-icon` (das Icon-Feld im Modal-Kopf) bleibt | Der Spec nennt im Modal-Kopf nur Titel und Schließ-Button. Das Icon-Feld trägt bei der Bestätigung und beim Produkt-Scan die Gefahrenstufe, und der Feedback-Dialog ist als Toast ersetzt — das Icon trägt jetzt die Warnung, die das Icon-Feld vorher getragen hat. |
| 23 | Die Spec-Sheets zeichnen ihre Komponenten selbst | Die Sheets binden die kanonischen Tokens und das kompilierte App-Stylesheet, tragen aber eine eigene Zeichnung der Bootstrap-Komponenten. Ein Button im Sheet ist nicht der Button der App — er sieht nur so aus. Die Sheets sind Mockups; wer die echten Komponenten sehen will, fährt die App an. |
| 24 | Die App hat keine Sidebar, die Navigation sitzt in der Topnav | `nav.md` schrieb die sieben primären Einträge in eine 240-px-Sidebar — die Spec-Sheets zeigen sie als waagerechte Linkzeile in der Topnav. Die App folgt den Sheets: Topnav mit Wortmarke, Linkzeile und Utility-Cluster, darunter der Inhalt. Die drei sekundären Ziele (Lizenzen, Quelle, Version) liegen in einer Fußzeile unter dem Inhalt. |
## Geprüfte Werte

`task design-check` misst diese Paare und bricht ab, wenn eines unterschritten
wird:

| Paar | Ziel |
|---|---|
| `--fg`, `--fg-2`, `--muted`, `--meta` auf `--bg` | ≥ 4.5:1 |
| `--meta` auf `--surface` | ≥ 4.5:1 |
| `--accent-on` auf `--accent` | ≥ 4.5:1 |
| `--accent-text` auf `--bg` und auf `--accent-soft` | ≥ 4.5:1 |
| `--accent` auf `--bg` | ≥ 3:1 (Indikator) |
| `--status-*-text` auf `--bg`, auf `--surface` und auf dem eigenen 18-%-Chip | ≥ 4.5:1 |
| `--status-*` auf `--bg` | ≥ 3:1 (Indikator) |

Alle Werte gelten für das helle **und** das dunkle Theme.

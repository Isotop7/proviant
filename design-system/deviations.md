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
| 25 | Die Panels tragen die Seitenfarbe: `--surface` ist im hellen Theme die Crème des Papiers | Die Spec-Sheets zeichnen ihre Karten, Eingaben und Panels in der Seitenfarbe — sie koppeln `--surface` an `--bg`. Das kanonische Token folgt ihnen: im hellen Theme ist `--surface` die Crème selbst, im dunkten bleibt es die gehobene Fläche (`#2C2823`), damit die Panels sich dort vom Papier lösen. Der Rand und der warme Schatten tragen die Trennung im hellen Theme. |
| 26 | Die Buttons folgen den Spec-Sheets, nicht `DESIGN.md` §5 | Die Sheets zeichnen 44 px, `--text-sm` und Versalien; `DESIGN.md` §5 verlangt 40 px, `--text-base` und Sentence-Case. Bis die Norm nachgezogen ist, gilt die Zeichnung der Sheets als Referenz: die App folgt ihr, die Button-Tabelle in §5 ist damit überholt und wird in einem späteren Pass ersetzt. |
| 27 | Die Produkttabelle steht in 900 px statt in der globalen Content-Breite | `DESIGN.md` §4 nennt 1120 px als maximale Content-Breite; die Spec-Sheets geben den Tabellenscreens 900 px, damit die Spalten eine lesbare Zeilenlänge behalten. Die Sheets sind die normative Schicht für die Anordnung, also misst die Produkttabelle 900 px; alle übrigen Screens behalten 1120 px. |
| 28 | Das Formular-Panel des Auth-Screens steht links, das Brand-Panel rechts | `DESIGN.md` §6 schreibt das Brand-Panel nach links und das Formular nach rechts. Die Spec-Sheets zeichnen es umgekehrt, und die App folgt ihnen, bis die Norm nachgezogen ist. |
| 29 | Die Eingabefelder sind 44 px hoch | `DESIGN.md` §5 nennt 40 px, die Spec-Sheets zeichnen 44 px — die Höhe, die auch die Mindestberührungsfläche der App verlangt. Die App folgt den Sheets. |
| 30 | Die Einstellungen stehen in der Topnav, nicht im Mobile-More-Menü | `nav.md` legt die Einstellungen in das More-Menü. Auf dem Telefon ist das More-Menü zwei Klicks entfernt und die Einstellungen stehen rechts in der Topnav, wo sie mit einem Klick erreichbar sind. Die Sheets zeichnen die Einstellungen ebenfalls nicht im More-Menü. |
| 31 | Die Ablauf-Liste nennt nur den Produktnamen, keinen Ort und keine Menge | Das Sheet zeichnet unter dem Namen eine Meta-Zeile ("Fridge · 250g"). Die Stats-API liefert pro Produkt nur `productName` und `expireAt`; Ort und Packungsgröße müssten über `StatsExpiringProduct` nachgezogen werden, was auch den Backup-Export berührt. Der Streifen, das Datum und der Status-Chip stehen trotzdem in der Zeile. |
| 32 | Die Kachel "Zuletzt hinzugefügt" sagt "Zuletzt hinzugefügt", nicht "Heute hinzugefügt" | Das Sheet nennt das Tagesdatum. Die Stats-API liefert nur den Produktnamen, kein Datum — ein erfundenes Datum wäre eine falsche Aussage. |
| 33 | Das Dashboard hat keine Kachel "Low stock" und keine Kachel "Rezeptideen" | Das Sheet zeichnet beide. Für die Low-Stock-Zahl und die Zahl der nutzbaren Rezepte gibt es keinen Endpoint; das Blenden der Kacheln mit erfundenen Zahlen ist keine Option. |
| 34 | Die gefüllte Verfalls-Kachel nutzt das Token-Paar statt des Oklch-Overrides des Sheets | Das Sheet überschreibt die gefüllte Fläche im dunklen Theme mit einem rohen `oklch(30% .06 25deg)`. Die App trägt `--status-expired` als Fläche und `--status-expired-on` als Tinte; das Token-Paar ist die Regel des Specs und braucht kein Theme-Override. |
| 35 | Der Frische-Chip im Hero zählt Produkte ohne Ablaufdatum mit | Die Stats-API kennt nur "abgelaufen", "bald fällig" und den Bestand. Der Chip ist die Differenz aus Bestand und diesen beiden Gruppen; Produkte ohne Datum tragen kein Ablaufdatum und fallen in die Differenz. |
| 36 | Die Gefahrenzone nutzt das Token-Paar statt der Oklch-Werte des Sheets | Das Sheet zeichnet die Karte mit `oklch(0.88 0.03 25)` (Rahmen), `oklch(0.98 0.01 25)` (Kopf) und `oklch(0.95 0.04 25)` (Icon-Chip). Die App trägt `--status-expired-subtle` (18 %) als Fläche, `--status-expired-border` (32 %) als Rahmen und einen 26-%-Mix für das Icon-Chip. Gemessen auf der 18-%-Fläche: Überschrift `--status-expired-text` 5.50:1 (hell) / 4.51:1 (dunkel), Sekundärtext `--fg-2` 6.45:1 / 7.39:1. `--muted` fällt dort auf 3.88:1 / 4.33:1 und trägt in der Zone daher keinen Text. |
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

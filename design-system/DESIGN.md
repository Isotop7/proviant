# Design System — proviant

> Category: Lifestyle · Haushalt
> Domain: Web (responsive, mobile-first), 360 – 1920 px
> Stand: 2026-06

proviant hilft Privatpersonen und Haushalten, ihren Lebensmittelvorrat zu
verwalten, Müll zu reduzieren und Einkäufe zu planen. Das Design-System
liefert die verbindliche Sammlung aus Token, Komponenten und Patterns,
mit der das proviant-Frontend in `src/templates/web` und `src/components`
einheitlich umgesetzt wird.

## 1. Markengefühl

proviant ist **kein Dashboard**, sondern ein **Vorratsregal**. Das System
fühlt sich an wie ein warmes Küchen-Notizbuch auf Crème-Papier: ruhig,
vertrauenswürdig, alltagstauglich. Die UI tritt zurück; der Inhalt
trägt — genauso wie das aktuelle Auth-Template (`baseAuth.tmpl`).

Drei Sätze Identität:

1. **Warmes Crème, kein Tech-Weiß.** Crème-Papier als Grundton, Tinte in
   Anthrazit, ein einziger Salbei-Akzent für Aktion und Fortschritt.
2. **VendSans trägt, DMMono zählt.** VendSans (Variable 300 – 700) für
   UI und Fließtext. DMMono nur dort, wo Zahlen, Codes oder Bestände
   ausgerichtet werden müssen.
3. **Vierstufiger Vorratsstatus.** `fresh`, `soon`, `critical`, `expired`
   sind die zentrale Sprache der App. Sie sind die einzigen Farben, die
   außer dem Akzent hervortreten dürfen.

## 2. Farbpalette

### Schema-Slots (verbindlich)

| Slot | Hex | Rolle |
|---|---|---|
| `--bg` | `#FAF7F1` | Crème-Papier, App-Hintergrund |
| `--surface` | `#FAF7F1` | Karten, Panels, Inset-Container |
| `--fg` | `#2A2A2A` | Primäre Tinte, Überschriften, Body |
| `--muted` | `#6B6B6B` | Sekundärtext, Meta, Labels |
| `--border` | `#E5DFD3` | Standard-Rahmen, Trennlinien |
| `--accent` | `#418069` | Salbei — primäre Aktion, Fortschritt, Logo |

`--accent` ist funktional, niemals dekorativ. Es darf **maximal zweimal
sichtbar pro Screen** auftreten (typisches Paar: ein primärer Button +
ein Status-Chip ODER eine Navigations-Pille + ein Fortschrittsbalken).

### Status-Quadrupel (Vorratssprache)

| Token | Hex | Bedeutung |
|---|---|---|
| `--status-fresh` | `#418069` | Haltbarkeit > 7 Tage |
| `--status-soon` | `#A58529` | 3 – 7 Tage |
| `--status-critical` | `#C26A3F` | 1 – 2 Tage |
| `--status-expired` | `#A24A4A` | abgelaufen |
| `--status-neutral` | `#8A8A8A` | kein Ablaufdatum |

Status ist **nicht** `--accent`. Status darf in Listen, Tabellen und
Karten mehrfach nebeneinander stehen, ohne dass es als "mehrere
Akzente" zählt.

**Basis- und Textvariante.** Die Basisfarben tragen Icons, Punkte, Balken
und Rahmen; **Text** nimmt die dunklere `--status-*-text`-Variante. Ohne
diese Trennung fällt die Beschriftung auf dem 18-%-Tint unter 4.5:1 —
`soon` läge bei 2.18:1. Jede Basisfarbe erreicht mindestens 3:1 gegen
`--bg`, damit ein Statuspunkt als Zustandsanzeige sichtbar bleibt.

### Erweiterte Slots (semantisch)

- `--surface-warm` `#F1ECE0` — wärmere Karte (z. B. Auth-Brand-Panel).
- `--fg-2` `#4A4A4A` — sekundäre Tinte, leicht heller als `--fg`.
- `--meta` `#6F6F6F` — tertiärer Text, Zeitstempel, Captions.
- `--border-strong` `#D5CDB8` — verstärkter Rahmen für fokussierte Felder.
- `--accent-soft` `#DDE7E0` — Salbei getintet, für Hover-BG und Chips.
- `--accent-text` `#35604E` — **Text** in Akzentfarbe (Links, aktive
  Navigation). `--accent` selbst ist die Flächenfarbe; als Text erreicht
  es auf `--bg` nur 4.35:1 und auf `--accent-soft` 3.67:1.
- `--accent-on` `#FFFFFF` — Text auf Salwei.
- `--shadow-warm` `rgba(80, 60, 30, 0.08)` — warmer Sepia-Schatten,
  niemals neutral-grau.
- `--success` `#418069` (Aliase zu `--status-fresh`).
- `--warn` `#C2A24A` (Aliase zu `--status-soon`).
- `--danger` `#A24A4A` (Aliase zu `--status-expired`).
- `--info` `#5A7A8A` — Hinweise (z. B. "neues Rezept verfügbar"), gedeckt.

### Abgeleitete Slots

Keine eigenen Farbwerte, sondern Ableitungen aus den Slots darüber. Sie stehen
als Ausdruck in der Quelle, nicht als Farbe — dadurch ziehen sie mit, wenn
`--surface` im Dark Theme wechselt:

```
--status-*-subtle:  color-mix(in oklab, var(--status-*) 18%, var(--surface));
--status-*-border:  color-mix(in oklab, var(--status-*) 32%, var(--surface));
--status-*-hover:   color-mix(in oklab, var(--status-*), black 8%);
--accent-hover:     color-mix(in oklab, var(--accent), black 6%);
--accent-active:    color-mix(in oklab, var(--accent), black 12%);
--accent-border:    color-mix(in oklab, var(--accent) 45%, var(--surface));
--status-*-on:      Text auf einer vollen Statusfläche (hell: Weiß, dunkel: Tinte)
```

Der Akzent-Rahmen mischt den Akzent mit der **Fläche** statt mit Weiß: im
hellen Theme hellt er auf, im dunklen dunkelt er ab.

### Was die Palette nicht enthält

- Kein Indigo, kein Violett, keine "Trust"-Gradients.
- Kein reines Weiß als Hintergrund (nur als Kartenoberfläche).
- Kein reines Schwarz für Text.
- Keine Emojis als Status- oder Feature-Icons.
- Keine zweite Akzentfarbe. `proviant` ist Salwei + Anthrazit.

## 3. Typografie

### Schriften

- **Display / UI / Body** — `VendSans` (Variable, 300 – 700),
  Fallback `system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", sans-serif`.
- **Mono** — `DMMono` (Regular 400, Medium 500),
  Fallback `ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, monospace`.

VendSans trägt alles, was im Interface lesbar sein muss. DMMono wird
**ausschließlich** für Bestandszahlen, Datums-Shortcodes, Hash-Looks
und Code-Beispiele eingesetzt.

### Skala

| Rolle | Token | VendSans | Größe | Zeilenhöhe | Tracking |
|---|---|---|---|---|---|
| Display | `--text-display` | 600 | 32 px | 1.15 | −0.01 em |
| H1 | `--text-h1` | 600 | 24 px | 1.20 | −0.01 em |
| H2 | `--text-h2` | 600 | 20 px | 1.25 | 0 |
| H3 | `--text-h3` | 500 | 18 px | 1.30 | 0 |
| Body | `--text-base` | 400 | 15 px | 1.55 | 0 |
| Body Bold | `--text-base-bold` | 600 | 15 px | 1.55 | 0 |
| Small | `--text-sm` | 400 | 13 px | 1.45 | 0 |
| Caption | `--text-xs` | 500 | 11 px | 1.40 | +0.04 em |
| Eyebrow | `--text-eyebrow` | 600 | 11 px | 1.20 | +0.10 em (uppercase) |
| Mono | `--text-mono` | DMMono 400 | 13 px | 1.40 | 0 |

`Eyebrow` ist uppercase mit `letter-spacing: 0.10em` und
`text-transform: uppercase`. Es leitet Sektionen ein (z. B. "VORRAT",
"AKTIVITÄT", "REZEPTE"). Body bleibt ohne Tracking.

### Drei-Gewicht-System

- **400** Body, Listen, Formularfelder.
- **500** H3, fette Labels, Captions.
- **600** Display, H1, H2, Eyebrow, Buttons.

700 wird **nicht** im Default-Set verwendet; VendSans Variable darf
hochgezogen werden, wenn ein Hero besonders betont werden muss.

### Linienlänge

Body-Text auf max. `65ch`. Listen und Datentabellen dürfen dichter
laufen (kein ch-Limit, aber `--text-sm` 13 px).

## 4. Spacing & Layout

### Spacing-Skala (4-px-Basis)

| Token | Wert | Verwendung |
|---|---|---|
| `--space-1` | 4 px | Inline-Gap, Icon-Padding |
| `--space-2` | 8 px | enges Stack-Glied |
| `--space-3` | 12 px | Form-Feld-Innenpadding vertikal |
| `--space-4` | 16 px | Standard-Innenpadding Karte |
| `--space-5` | 20 px | geräumige Karte |
| `--space-6` | 24 px | Sektions-Innenpadding |
| `--space-8` | 32 px | Sektion-Spacer (Desktop) |
| `--space-10` | 40 px | Hero-Spacer (Desktop) |
| `--space-12` | 48 px | Seitenrand groß |
| `--space-16` | 64 px | Page-Top-Padding Desktop |

### Radius

| Token | Wert | Verwendung |
|---|---|---|
| `--radius-sm` | 4 px | Badges, Tags, kleine Chips |
| `--radius-md` | 8 px | Standard-Karten, Inputs |
| `--radius-lg` | 12 px | Hero-Karten, Modals |
| `--radius-pill` | 999 px | Pill-Buttons, Tag-Filter |

Modals und Toasts: `--radius-lg`. Listen-Items: kein Radius (flache
Trennlinien). Inputs: `--radius-md`.

### Schatten (warm, nie neutral)

| Token | Wert | Verwendung |
|---|---|---|
| `--elev-flat` | `none` | Karten im Ruhezustand |
| `--elev-raised` | `0 1px 2px rgba(80,60,30,0.06), 0 4px 12px rgba(80,60,30,0.08)` | Hover-Karten, Dropdowns |
| `--elev-overlay` | `0 8px 24px rgba(80,60,30,0.12), 0 24px 48px rgba(80,60,30,0.10)` | Modals, Toasts, Menüs |
| `--elev-inset` | `inset 0 0 0 1px var(--border)` | Fokussierte Felder, ausgewählte Listen-Items |

Schatten tragen **Sepia**, kein Neutral-Grau. Auf Crème-Papier
wirkt neutral-grau kalt.

### Layout-Prinzipien

- Mobile-first. 360 px ist der Nullpunkt, nicht 1024 px.
- Maximale Content-Breite 1120 px. Darüber zentriert mit `--space-12` Rand.
- 12er-Spalten-Grid auf Desktop, 4er auf Mobile.
- Sidebar: 240 px fix auf Desktop, Drawer unter 1024 px.
- Topbar: 56 px hoch, 1 px `--border` unten, keine Schatten-Linie.
- Sektions-Rhythmus: abwechselnd `--bg` und `--surface-warm` (nie
  weiß-auf-weiß, nie grau-auf-crème).

## 5. Komponenten

**Basis: Bootstrap 5.3.** Jede Komponente wird primär über Bootstrap-Standardklassen (`.btn`, `.form-control`, `.card`, `.table`, `.list-group`, `.badge`, `.modal`, `.toast`, `.nav`, `.spinner-border`, `.alert`) umgesetzt. Die Marken-Palette wird über `--bs-*`-Variablen in `tokens.css` eingespielt (siehe § 2.5), sodass alle Bootstrap-Komponenten die Salwei-Farben tragen, ohne dass wir pro Komponente BG- und FG-Werte neu setzen müssen.

Eigene Klassen werden **nur** dort eingeführt, wo Bootstrap keine passende Komponente hat:
- Layout-Patterns (`.app`, `.auth`, `.app__sidebar`, `.bottom-nav`, `.kpi`, `.skeleton`, `.field--code`)
- Marken-Modifier auf Bootstrap-Komponenten (`.badge--fresh/soon/critical/expired/neutral`, `.toast--info/success/warn/danger`, `.kpi--focus`, `.card--hero`, `.field--search`, `.modal--demo`)
- Layout-Helfer (`.eyebrow`, `.stack-*`, `.page-header`, `.page-nav`)

Das Mapping der früheren eigenen Klassen auf Bootstrap-Klassen ist in `components.manifest.json` unter `classMapping` festgehalten und verbindlich — wenn eine Komponente im Repo über Bootstrap realisiert werden kann, MUSS sie über Bootstrap realisiert werden. Ausnahmen sind mit Maintainer-Begründung im PR zu dokumentieren.

Nicht verwendet werden: `navbar`, `dropdown-menu`, `carousel`, `btn-toolbar`, `form-floating` — sie passen visuell nicht zur Pill-/Crème-Sprache. Begründung pro Komponente in `components.manifest.json#bootstrapReplaced`.

### Seitenkopf (Page Header)

Der Seitenkopf trägt den Namen der Seite — er führt den **einzigen** `h1`
der Seite (die Topnav ist Navigation). Er ist **kein Band**: keine Fläche,
keine Linie, kein Schatten, kein Bleed. Er besteht aus vier Teilen:

| Teil | Klasse | Inhalt |
|---|---|---|
| Name | `.eyebrow` (als `h1`) | der Seitenname, klein, Versalien, `--muted` |
| Zahl | `.page-header__count` | die Zahl, die die Seite zählt — `--text-display` 600, tabellarische Ziffern, `--tracking-tight` |
| Erklärung | `.page-header__sub` | was die Zahl zählt, `--text-sm`, `--fg-2` |
| Kontext | `.page-header__meta` | rechtsbündig, klein, Versalien, `--muted` (z. B. „2 EXPIRED · 5 EXPIRING SOON“) |
| Aktionen | `.page-header__aside` | rechts unter dem Kontext, an der Unterkante bündig |

Seiten ohne Zahl (Einstellungen, Lizenzen, Profil, Rezepte) führen statt
dessen `.page-header__title` (`--text-h1` 600, `--tracking-tight`) mit
`.page-header__sub`.

Die Zahl kommt aus den Daten der Seite: Products, Shopping List, Waste
Analytics und Meal Plan zählen; Recipes, Einstellungen, Lizenzen und das
Profil nicht.

### Buttons

Bootstrap-`.btn` mit Salwei-Politur (Uppercase, Pill, Salwei-Fokus). Mapping:

| Früher (eigene Klasse) | Jetzt (Bootstrap) | Salwei-Politur |
|---|---|---|
| `.btn.btn--primary` | `.btn.btn-primary` | `--bs-btn-bg: var(--accent)` |
| `.btn.btn--secondary` | `.btn.btn-outline-secondary` | BG `--surface`, Border `--border` |
| `.btn.btn--ghost` | `.btn.btn-link` | transparent, Hover-BG `--accent-soft` |
| `.btn.btn--danger` | `.btn.btn-danger` | `--bs-btn-bg: var(--status-expired)` |

| Variante | BG | FG | Border | Radius | Tracking |
|---|---|---|---|---|---|
| `primary` | `--accent` | `--accent-on` | — | `--radius-pill` | 0.04 em |
| `secondary` (outline) | `--surface` | `--fg` | 1 px `--border` | `--radius-pill` | 0.04 em |
| `ghost` (link) | transparent | `--fg` | — | `--radius-pill` | 0.04 em |
| `danger` | `--status-expired` | `#FFF` | — | `--radius-pill` | 0.04 em |

Höhe: 40 px (Default), 48 px (Hero), 32 px (kompakt, in Tabellen).
Label: `--text-base` 600, `letter-spacing: 0.04em`, **Sentence-Case**. Die
frühere Uppercase-Regel kompensierte einen Kontrastmangel des alten Akzents
(`#5A8A7A`, 3.92:1 auf Weiß). Mit `--accent` #418069 erreicht eine 15-px-600-Beschriftung
4.65:1 und erfüllt WCAG AA ohne Großschreibung — und im deutschen Katalog
bleiben Komposita lesbar. Großbuchstaben sind dem `.eyebrow`-Label
vorbehalten. Hover: BG um 6 % abdunkeln via
`color-mix(in oklab, var(--accent), black 6%)`. Disabled: BG `--border`,
FG `--muted`, kein Pointer. **Für Secondary/Ghost/Danger ist der
Disabled-Zustand explizit zu setzen** (sonst kollabiert die Pill-Form).
Loading: `aria-busy="true"` + 14 px Spinner im Button-Inner, Padding
bleibt gleich. Fokus: `:focus-visible` setzt `box-shadow: var(--focus-ring)`
(3 px `--accent-soft`-Ring), Browser-Outline ist auf `none`.

### Formulare

Bootstrap-`.form-control` / `.form-label` / `.form-select` / `.form-text`. Status über `.is-invalid` + `.invalid-feedback` und `.is-valid` + `.valid-feedback` (Bootstrap-Standard-Pattern). Mapping:

| Früher | Jetzt (Bootstrap) |
|---|---|
| `.field__label` | `.form-label` |
| `.field__input` | `.form-control` |
| `.field__select` | `.form-select` |
| `.field__textarea` | `.form-control` mit `rows="…"` |
| `.field__error` | `.invalid-feedback` (zusammen mit `.is-invalid` auf `.form-control`) |
| `.field__success` | `.valid-feedback` (zusammen mit `.is-valid` auf `.form-control`) |
| `.field__hint` | `.form-text` |
| `.field--code` | `.field--code` (eigene Helfer-Klasse, da Bootstrap keine 6-stelligen Code-Inputs kennt) |

- Feldhöhe 40 px, Innenpadding `--space-2` `--space-3`.
- Label oberhalb (nicht links), `--text-sm` 500, `--fg`.
- Placeholder `--meta`.
- Fokus-Border: 1 px `--accent` + `--focus-ring`-Shadow.
- Fehler: 1 px `--status-expired`, Hilfetext darunter in derselben Farbe (`role="alert"`).
- Erfolg: 1 px `--status-fresh`, Icon `bi-check-circle` + Text in `--status-fresh` (z. B. "Erkannt und dem Vorrat zugeordnet"). Wird nur nach echter Server-Bestätigung gesetzt, nicht optimistisch.
- Helper: `--meta`, optional Icon links (`bi-info-circle`).
- Pflichtfeld-Markierung: Asterisk nach Label in `--status-expired`.

Wichtige Felder:

- **Text / Email / Passwort** — `--radius-md`, BG `--surface`.
- **Suche** — pill, Icon links (`bi-search`), X-Button rechts zum Leeren.
- **Select** — nativer `<select>` ist okay; Custom-Trigger zeigt Chevron rechts.
- **Datum** — nativer Datepicker; VendSans im Eingabefeld.
- **Textarea** — Mindesthöhe 96 px, resize vertikal.

### Karten

- BG `--surface`, Border `1 px --border`, Radius `--radius-md`.
- Innenpadding `--space-4` (Default), `--space-5` (Hero).
- Hover: `--elev-raised`, BG bleibt `--surface`.
- Header optional: Trennlinie `1 px --border` darunter, Padding `--space-3` `--space-4`.
- Footer optional: gleiche Trennlinie, Buttons rechtsbündig.

### Listen

Bootstrap-`.list-group` mit Custom-Geometrie (56 px Min-Höhe, 40 px Leading-Avatar). Mapping: `.list` → `.list-group`, `.list__item` → `.list-group-item`, `.list__item--active` → `.list-group-item.active`. Eigene Sub-Slots `.list-group-item__leading/body/title/meta/trailing` für Avatar + Body-Layout (Bootstrap liefert das nicht).

- Listen-Item: keine Karte, sondern Zeile mit `1 px --border` darunter.
- Höhe 56 px (Default), 40 px (kompakt), 72 px (Hero mit Bild).
- Leading: Bild / Icon 40 × 40, dann Titel (`--text-base` 600), darunter Meta (`--text-sm` `--muted`).
- Trailing: Status-Chip oder Aktion-Button.
- Swipe-Action nur auf Mobile (nicht im Web-Default).

### Tabellen

- Header: `--text-xs` 600 uppercase, `--muted`, Tracking 0.04em.
- Zell-Padding `--space-3` `--space-4`.
- Trennlinien `1 px --border`, niemals Schatten-Linien.
- Sortierbarer Header: Chevron rechts (`bi-chevron-expand`).
- Zeilen-Hover: BG `--accent-soft`, FG bleibt `--fg`.
- Auswahl: Checkbox links, ausgewählte Zeile BG `--accent-soft` + Border-Left `2 px --accent`.
- Pagination unten rechts, "1 – 10 von 84" links, VendSans 400.
- **Leerer Zustand**: `.table-wrap` enthält nur einen `.table-empty`-Block (zentriert, Icon `bi-inbox` 32 px in `--meta`, Headline + CTA "Filter zurücksetzen" oder "Erstes Produkt anlegen"). Die Header-Zeile verschwindet — Empty ersetzt die Tabelle vollständig, nicht nur den Body.

### Datentabellen (dicht)

- Höhe 32 px pro Zeile, Padding `--space-2` `--space-3`.
- Mono-Spalten (`--text-mono`) für Zahlen, rechtsbündig.
- Sticky-Header beim Scrollen.
- Resize-Handle zwischen Spalten (4 px breiter Cursor-Streifen).

### App-Layout / Topnav

Eigene Layout-Komponente (Bootstrap hat keine passende App-Shell). Die Navigation trägt die Topnav: Wortmarke links, die sieben primären Einträge in der Mitte, der Utility-Cluster rechts. Bottom-Nav nutzt `.nav` mit Grid-Layout (5 Spalten) und `env(safe-area-inset-bottom)`.

Verbindliche Menü-Struktur (sieben Topnav-Items,
Footer-Utilities, Bottom-Nav-Slots) steht in `design-system/nav.md`.

- Topnav 56 px fix, sticky; die Linkzeile verschwindet < 1024 px, dann übernimmt der Bottom-Nav.
- BG `--surface` zu 85 % mit Blur, Trennlinie unten `1 px --border`.
- Wortmarke links, Linkzeile zentriert, Utility-Cluster rechts.
- Link: `--text-sm` 500, FG `--fg-2`, Radius `--radius-sm`.
- Aktiv: BG `--accent-soft`, FG `--accent-text`, Font-Gewicht 600.
- Inaktiv: FG `--fg-2`, Gewicht 400.
- Die Seite trägt ihren Namen selbst: das `.page-header` führt das **einzige** `h1`, die Topnav ist Navigation.
- **Theme-Umschalter: genau einer pro Seite.** In der Shell im Topbar-Utility-Cluster (Desktop) bzw. im Mobile-More; in den Spec-Screens ebenfalls im Topnav-Utility-Cluster (`.nav-actions`). Ein zusätzlicher Floating-Button unten rechts ist verboten. Auth-Screens haben keinen Umschalter — dort greift nur `data-bs-theme` / `prefers-color-scheme`.
- **Mobile-Bottom-Nav** (sichtbar < 1024 px, ersetzt Sidebar): BG `--surface-warm`, Border oben 1 px `--border`, 5 Grid-Spalten (Dashboard · Products · Scan · Shopping · Recipes), 56 px hoch + `env(safe-area-inset-bottom)` für iOS-Home-Indikator. Item: Icon 20 px + Label 11 px, aktiv in `--accent` auf `--accent-soft`. Meal Plan, Waste Analytics und Settings liegen im Mobile-More-Menü.

### Modals

Bootstrap-`.modal` mit Salwei-Politur. In der echten App via Bootstrap-JS aktiviert (`data-bs-toggle="modal"`, `data-bs-target="#…"`). Im Spec-Sheet steht ein `.modal--demo` mit `.show .d-block`, damit das Modal im Screenshot sichtbar bleibt.

- Overlay `rgba(40, 30, 20, 0.45)` (warmes Anthrazit, nicht Schwarz).
- Karte BG `--surface`, Radius `--radius-lg`, Schatten `--elev-overlay`.
- Padding `--space-6`.
- Header: Titel (`--text-h2` 600), Close-Button rechts (`bi-x-lg`).
- Footer: Buttons rechtsbündig, Cancel links in `--text-muted`, Primary rechts.
- Max-Breite 480 px (Form), 720 px (Detail), 960 px (Vollbild).
- **In der App-Implementierung** ist `.modal__overlay` `position: fixed; inset: 0` (Fullscreen-Backdrop, z-index 50). Das Spec-Sheet zeigt das Modal in einem relativen Demo-Container, damit es im Screenshot sichtbar bleibt.

### Toasts

Bootstrap-`.toast` mit Status-Modifiern (`.toast--info|success|warn|danger`). Mapping: `.toast` bleibt, Status-Border-Left über eigene Modifier, Icon und Body-Slots über `.d-flex` + `.gap-*` (Bootstrap-Standard).

- Position: oben rechts, 16 px vom Rand, Stapel mit `--space-2`-Abstand.
- Karte: BG `--surface`, Border links `3 px` (Status-Farbe), Radius `--radius-md`, Schatten `--elev-overlay`.
- Dauer: 4 s (Info), 6 s (Warn), persistent (Error mit Dismiss-Button).
- Icon links, Titel (`--text-base` 600), Body (`--text-sm` `--muted`).

### Feedback / Empty States

- Empty: zentriertes Icon (`bi-basket` o. ä.) 48 × 48 in `--meta`, Headline + Helper darunter, primärer Button (`.btn.btn-primary`).
- Loading: Bootstrap-`.spinner-border` mit `.text-primary` für Vollflächen, eigener `.spinner` (24 × 24, `@keyframes spin`) für Inline-Loading in Buttons. Salwei-Top-Edge.
- Skeleton: `linear-gradient(90deg, --surface-warm, --border, --surface-warm)`, 1.4 s Loop. Bootstrap hat keinen Skeleton-Primitive; die `.skeleton`-Klasse ist die einzige eigene Lösung, wo Bootstrap nichts Passendes liefert.

### Vorratsstatus (Chips/Badges)

Bootstrap-`.badge` mit Status-Modifiern. Mapping: `.chip` → `.badge`, Status-Farben über `.badge--fresh|soon|critical|expired|neutral` (eigene Modifier, da Bootstrap-`.bg-success/warning/danger` zu kräftig für die Crème-Marke sind — wir nutzen `color-mix(18%)` für getintete BGs).

- Vier Stufen: `fresh` (Salbei, > 7 Tage), `soon` (Senf, 3 – 7 Tage), `critical` (Terracotta, 1 – 2 Tage), `expired` (warmes Rot, abgelaufen). `neutral` (grau) für Produkte ohne Ablaufdatum.
- Hintergrund: `color-mix(in oklab, var(--status-x) 18 %, var(--surface))` — rechnet sich im Dark Theme mit, ohne eigene Token.
- **Text: `var(--status-x-text)`**, nicht die Basisfarbe. Die Basisfarbe bleibt für Icon, Punkt, Balken und Rahmen.
- Immer Farbe + Icon (`bi-check-circle` / `bi-clock` / `bi-exclamation-triangle` / `bi-x-circle`) + Text.

### Dashboard / Kennzahlen-Kacheln

Bootstrap-`.card` (mit `.col-12 .col-md-6 .col-lg-3` für 1/2/3/4 Spalten je Breakpoint) + eigenes `.kpi`-Layout-Pattern (`.kpi__label`, `.kpi__value`). `.kpi__delta` und `.kpi__sparkline` sind **nicht** Teil des Systems — die Stats-API liefert pro Kachel keine Historie (siehe `deviations.md` #3).

- Karten-Layout 1 / 2 / 3 / 4 Spalten je Breakpoint.
- Kachel-Inhalt: Eyebrow + große Zahl (`--text-display` VendSans 600) + Delta darunter (`--text-sm`, Salwei-Pfeil nach oben, Terracotta-Pfeil nach unten) + optionaler Sparkline 80 × 24.
- Eine Kachel darf `--accent-soft` als BG haben (`.kpi--focus`), um sie als "im Fokus" zu markieren. Maximal eine pro Dashboard.

### Diagramme / Charts

- Bibliothek: **Chart.js** (Default) oder **ApexCharts** (für komplexe
  Tooltips). Kein D3 im Default, da Overhead zu hoch für Haushalts-UI.
- Primärfarbe: `--accent`. Sekundärfarben: `--status-fresh`, `--status-soon`, `--status-critical`, `--status-expired`. Keine weiteren Farben.
- Achsen: VendSans 13 px 400, FG `--muted`. Trennlinien `1 px --border`.
- Tooltip: BG `--surface`, Border `--border`, Schatten `--elev-raised`.
- Legende: unterhalb des Charts, 12 px Gap, VendSans 13 px.
- Bestand-Linie immer Salwei (`--accent`), Trend-Linie `--status-soon` gestrichelt.

## 6. Auth (spezifisch)

Auth ist die prominenteste Fläche. Hier darf das System spürbar werden:

- **Brand-Panel** links auf Desktop (50 % Breite), BG `--surface-warm`.
  Logo oben links, Headline (`--text-display` 600, VendSans) zentriert
  vertikal, Bildplatzhalter (16:9) darunter.
- **Form-Panel** rechts auf Desktop, BG `--bg`, Padding `--space-12`.
  Felder `--space-5` vertikal gestapelt. Primärer Button full-width.
- **Mobile**: nur Form-Panel, Brand-Block oben (Logo + Wortmarke), dann
  Felder, dann Footer-Hinweis ("Du hast schon ein Konto? Anmelden").
- Verify- und Forgot-Screens: gleiche Form-Geometrie, andere Copy und
  ggf. ein Code-Input-Feld (6 Ziffern, Mono-Font, je 48 × 56 px). Jedes
  Code-Input bekommt `aria-label="Code-Ziffer N"` (1 – 6), damit
  Screenreader den Fokus pro Ziffer sprechen können.

## 7. Motion

- Easing: `cubic-bezier(0.23, 1, 0.32, 1)` für UI-Transitions.
- Dauer: 180 ms Enter, 120 ms Exit. Toasts 240 ms Enter.
- Reduce-Motion-Pflicht: `@media (prefers-reduced-motion: reduce)` setzt
  alle Dauern auf 0 ms und deaktiviert Skeleton-Loops.
- Spinner: 1 s linear infinite.
- Modal-Enter: scale `0.96 → 1` + opacity `0 → 1` über 180 ms.
- Toast-Enter: translateY `-8 px → 0` + opacity `0 → 1` über 200 ms.
- Listen-Reorder: 220 ms, FLIP-Pattern (Height + Transform gleichzeitig).

## 8. A11y

- Kontrast Body-Text (`--fg` auf `--bg`): 13.42 : 1 (AAA).
- Kontrast `--muted` auf `--bg`: 4.98 : 1 (AA Body).
- Kontrast `--meta` auf `--bg`: 4.70 : 1 (AA Body).
- Kontrast Akzent-Button (`--accent-on` auf `--accent`): 4.65 : 1 (AA Body).
- Kontrast `--accent-text` auf `--bg`: 6.69 : 1 (AAA).
- Status-Beschriftung (`--status-*-text` auf dem eigenen 18-%-Chip): 4.51 bis 6.41 : 1 (AA Body).
- Status-Indikator (`--status-*` auf `--bg`): 3.23 bis 5.43 : 1 (AA Non-Text).
  Alle Werte werden von `task design-check` gemessen und blockieren den Build,
  sobald ein Paar die Schwelle unterschreitet.
- Status-Farben sind nie **alleinige** Information. Immer zusätzlich
  Icon (`bi-check-circle`, `bi-exclamation-triangle`, `bi-x-circle`,
  `bi-clock`) und Text.
- Fokus: `outline: 2 px solid --accent; outline-offset: 2 px` auf
  allen interaktiven Elementen. Eigener `--focus-ring` Token:
  `0 0 0 3px var(--accent-soft)`.
- Tastatur: Tab springt in sichtbarer Reihenfolge, Esc schließt Modals
  und Drawer, Enter löst primäre Aktion aus, Space toggelt Checkboxen.

## 9. Do & Don't

### Do

- `--bg` für Seiten, `--surface` für Karten, `--surface-warm` für
  Hervorhebung. Niemals weiß-auf-weiß.
- VendSans für alles, DMMono nur für Zahlen.
- Status via Farbe + Icon + Text.
- Pill-Buttons (`--radius-pill`) für Aktionen, eckige Karten für Inhalt.
- Schatten in Sepia, nicht in Neutral-Grau.
- Großzügige Zeilenhöhe (1.5+) für Body.

### Don't

- Kein Indigo, kein Violett, kein "Trust"-Gradient.
- Keine Emojis als Feature-Icons.
- Keine Rounded-Card-mit-Left-Border-Slop (Status-Border links ist
  okay bei Toasts; bei normalen Karten ist es verboten).
- Kein Inter / Roboto / Arial als Display.
- Kein `box-shadow: 0 0 0 1px var(--border)` als alleinige Trennlinie
  (auf Crème-Papier zu schwach — `1 px solid` ist der Default).
- Kein `linear-gradient` auf Hero-BG außer dem expliziten Auth-Brand-Panel.
- Keine erfundenen Statistiken ("30 % weniger Müll") ohne Quelle.

## 10. Versionshinweis

Stand 2026-10.

**Die Token-Werte leben in `design-system/_tokens.scss`** (hell) und
`design-system/_tokens-dark.scss` (dunkt). Diese beiden Dateien sind die
einzige Quelle. `tokens.css`, `tokens.dark.css` und `tokens.json` sind
**generierte Artefakte** und werden von `task design-tokens` daraus
projiziert; `scripts/design-tokens.mjs --check` meldet veraltete
Artefakte. `task design-check` misst die Kontrastziele, prüfft die
Manifest-Zählwerte und schweigt nicht bei Drift.

Veränderungen laufen damit immer in dieser Reihenfolge:

1. `_tokens.scss` / `_tokens-dark.scss` anpassen
2. `task design-tokens` — erzeugt `tokens.css`, `tokens.dark.css`, `tokens.json`
3. `task design-check` — misst die neuen Werte
4. erst danach die Komponenten ändern

Früher lag hier eine `oklch()`-Spalte neben der Hex-Spalte, und die
beide widersprachen sich (beim Akzent um ΔL 0.086 und ΔH 9.3°).
Die Farbwerte stehen jetzt ausschließlich als Hex in der Quelle.

Menü-Struktur: Die Screens in `screens/` spiegeln die sieben
Sidebar-Items, die Topbar-Utilities und die fünf Bottom-Nav-Slots aus
`nav.md`. Die Screens sind eigenständige Spec-Sheets und tragen bis zur
Migration einen eigenen (älteren) Inline-Token-Block; ihre Anordnung ist
normativ, ihre Tokens nicht.

## 11. Dark Theme

Dark Mode ist opt-in: `<html data-bs-theme="dark">`, gesetzt von
`src/assets/js/theme.js`. Die Einstellung `system` wird vor dem ersten
Paint in ein explizites `light` oder `dark` aufgelöst; ohne JavaScript
greift `prefers-color-scheme: dark`.

- Tokens: `design-system/_tokens-dark.scss` (siehe `_tokens.scss` für die Struktur).
- Selbst-gehostete Fonts (VendSans / DMMono) bleiben unverändert;
  `font-display: swap` beibehalten.
- Komponenten erben Dark-Mode automatisch über CSS-Variablen — keine
  separate Dark-Komponente nötig.
- **Ein** Theme-Umschalter pro Seite (§ 5, App-Layout) — kein zweiter
  Trigger, insbesondere kein Floating-Button am unteren Rand. Auth-Screens
  bleiben ohne Umschalter.
- Der Akzent wird im Dark Theme auf `#59A388` gehoben: gleicher Farbton
  (168°), höherer Chroma, `--accent-on` wechselt auf `#1E1A14`.
  Damit erreicht er 5.79 : 1 gegen `--bg` und als Text-Farbe auf dem
  Akzent 5.79 : 1.
- Die Statusfarben behalten ihren Farbton und werden nur heller; die
  `--status-*-text`-Varianten wechseln mit.

## 12. Zwei Schichten, eine Wahrheit

Die beiden Artefakte in `design-system/` sind nicht gleichrangig, und sie
widersprachen sich bis zur Migration an drei Stellen. Die Auflösung:

| Schicht | Artefakt | Bindet |
|---|---|---|
| **System** | `DESIGN.md` (dieses Dokument) | Tokens, Farbe, Typografie, Spacing, Radius, Elevation, Motion, A11y, Status-Sprache, die Bootstrap-Klassen-Pflicht |
| **Komposition** | `screens/*.html` | die Anordnung eines Bildschirms: Kartenraster, Reihenfolge, Seitenaufteilung |

Die Trennlinie: **ändert die Regel das Aussehen einer Komponente oder die
Anordnung einer Seite?**

- *Komponente* (Farbe eines Buttons, Radius einer Karte, Schriftgröße einer
  Kachel) → `DESIGN.md` ist normativ.
- *Anordnung* (welche Karten auf dem Dashboard, wo das Brand-Panel, wie
  breit die Produktliste) → das Screen-Sheet ist normativ.

Wo ein Screen-Sheet eine Komponente zeigt, die `DESIGN.md` nicht kennt,
gewinnt `DESIGN.md`: die Komponente wird ergänzt oder als Abweichung in
`deviations.md` vermerkt — nie stillschweigend. Jede Abweichung hat dort
eine Begründung und, wenn sie sichtbar ist, einen messbaren Beleg.

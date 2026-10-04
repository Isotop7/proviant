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

| Slot | Hex | oklch | Rolle |
|---|---|---|---|
| `--bg` | `#FAF7F1` | `oklch(0.972 0.012 85)` | Crème-Papier, App-Hintergrund |
| `--surface` | `#FFFFFF` | `oklch(1 0 0)` | Karten, Panels, Inset-Container |
| `--fg` | `#2A2A2A` | `oklch(0.260 0.005 80)` | Primäre Tinte, Überschriften, Body |
| `--muted` | `#6B6B6B` | `oklch(0.515 0.005 80)` | Sekundärtext, Meta, Labels |
| `--border` | `#E5DFD3` | `oklch(0.905 0.018 80)` | Standard-Rahmen, Trennlinien |
| `--accent` | `#5A8A7A` | `oklch(0.508 0.062 162)` | Salbei — primäre Aktion, Fortschritt, Logo |

`--accent` ist funktional, niemals dekorativ. Es darf **maximal zweimal
sichtbar pro Screen** auftreten (typisches Paar: ein primärer Button +
ein Status-Chip ODER eine Navigations-Pille + ein Fortschrittsbalken).

### Status-Quadrupel (Vorratssprache)

| Token | Hex | oklch | Bedeutung |
|---|---|---|---|
| `--status-fresh` | `#5A8A7A` | `oklch(0.508 0.062 162)` | Haltbarkeit > 7 Tage |
| `--status-soon` | `#C2A24A` | `oklch(0.700 0.110 85)` | 3 – 7 Tage |
| `--status-critical` | `#C26A3F` | `oklch(0.605 0.140 50)` | 1 – 2 Tage |
| `--status-expired` | `#A24A4A` | `oklch(0.510 0.140 25)` | abgelaufen |

Status ist **nicht** `--accent`. Status darf in Listen, Tabellen und
Karten mehrfach nebeneinander stehen, ohne dass es als "mehrere
Akzente" zählt.

### Erweiterte Slots (semantisch)

- `--surface-warm` `#F1ECE0` — wärmere Karte (z. B. Auth-Brand-Panel).
- `--fg-2` `#4A4A4A` — sekundäre Tinte, leicht heller als `--fg`.
- `--meta` `#8A8A8A` — tertiärer Text, Zeitstempel, Captions.
- `--border-strong` `#D5CDB8` — verstärkter Rahmen für fokussierte Felder.
- `--accent-soft` `#DDE7E0` — Salbei getintet, für Hover-BG und Chips.
- `--accent-on` `#FFFFFF` — Text auf Salwei.
- `--shadow-warm` `rgba(80, 60, 30, 0.08)` — warmer Sepia-Schatten,
  niemals neutral-grau.
- `--success` `#5A8A7A` (Aliase zu `--status-fresh`).
- `--warn` `#C2A24A` (Aliase zu `--status-soon`).
- `--danger` `#A24A4A` (Aliase zu `--status-expired`).
- `--info` `#5A7A8A` — Hinweise (z. B. "neues Rezept verfügbar"), gedeckt.

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
Label: `--text-base` 600, immer uppercase mit `letter-spacing: 0.04em`.
Padding: `--space-2` `--space-4`. Hover: BG um 6 % abdunkeln via
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

### App-Layout / Sidebar

Eigene Layout-Komponente (Bootstrap hat keine passende App-Shell). Sidebar nutzt `.nav.flex-column` (Bootstrap-Nav) mit `.nav-link`-Items und Salwei-Politur. Bottom-Nav nutzt `.nav` mit Grid-Layout (4 Spalten) und `env(safe-area-inset-bottom)`.

- Sidebar 240 px fix auf ≥ 1024 px, Bottom-Nav < 1024 px.
- BG `--surface-warm`, Trennlinie rechts `1 px --border`.
- Section-Label: `--text-eyebrow`, Padding `--space-3` `--space-4`.
- Item: 40 px hoch, 8 px Radius, 12 px horizontaler Innenpadding.
- Aktiv: BG `--accent-soft`, FG `--accent`, Font-Gewicht 600.
- Inaktiv: FG `--fg-2`, Gewicht 400.
- Icon links 20 × 20, VendSans-Icon-Font oder Bootstrap-Icons.
- Topbar: 56 px, BG `--surface`, Border unten 1 px `--border`, Title links (`--text-h2` 600), Aktionen rechts.
- **Mobile-Bottom-Nav** (sichtbar < 1024 px, ersetzt Sidebar): BG `--surface-warm`, Border oben 1 px `--border`, 4 Grid-Spalten, 56 px hoch + `env(safe-area-inset-bottom)` für iOS-Home-Indikator. Item: Icon 20 px + Label 11 px, aktiv in `--accent` auf `--accent-soft`.

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

- Vier Stufen: `fresh` (Salwei, > 7 Tage), `soon` (Senf, 3 – 7 Tage), `critical` (Terracotta, 1 – 2 Tage), `expired` (warmes Rot, abgelaufen).
- Immer Farbe + Icon (`bi-check-circle` / `bi-clock` / `bi-exclamation-triangle` / `bi-x-circle`) + Text.

### Dashboard / Kennzahlen-Kacheln

Bootstrap-`.card` (mit `.col-12 .col-md-6 .col-lg-3` für 1/2/3/4 Spalten je Breakpoint) + eigenes `.kpi`-Layout-Pattern (`.kpi__label`, `.kpi__value`, `.kpi__delta`, `.kpi__sparkline`).

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

- Kontrast Body-Text (`--fg` auf `--bg`): 11.4 : 1 (AAA).
- Kontrast Muted (`--muted` auf `--bg`): 4.6 : 1 (AA Body).
- Kontrast Akzent-Button (`#FFF` auf `--accent`): 3.7 : 1 (AA Large).
  Für 14-px-Label ist das grenzwertig — Buttons bleiben deshalb immer
  uppercase und ≥ 14 px 600.
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

Stand 2026-06. Token-Werte spiegeln `main.scss` und `baseAuth.tmpl` aus
`/home/hendrik/dev/proviant/src/templates/web` zum Zeitpunkt der
Erstellung. Folgeänderungen am proviant-Code, die diese Werte
aktualisieren, müssen zuerst `tokens.css` und `tokens.json` anpassen,
bevor die Komponenten geändert werden.

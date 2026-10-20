# Spec-Sheets

21 eigenständige HTML-Dateien, die je einen Bildschirm des Design-Systems
zeigen. Sie sind **Referenz-Mockups**, kein Code und keine Testfälle — der
Beleg ist der Screenshot, nicht ein grüner Build.

## Stand der Migration

Die Sheets tragen aktuell noch ihre **eigene** `:root`-Token-Block-Definition
und ein selbst gebautes Bootstrap. `task design-check` meldet das ab Phase 6
als Fehler (`spec-sheets`).

Geplant (PR 6):

1. Der inline-`:root`-Block fällt weg, die Sheets binden `../tokens.css` — die
   generierte Projektion von `design-system/_tokens.scss`.
2. Die handgebaute Bootstrap-Reproduktion fällt weg, die Sheets binden das
   kompilierte `src/assets/css/main.css`. Damit rendern sie mit demselben
   Bootstrap, denselben Tokens und denselben Icons wie die App.
3. Die Klassen-Namen wechseln auf das App-Vokabular (`.btn btn-primary`,
   `.form-control`, `.list-group-item`, `.badge badge--*`, `.kpi`, `.toast`).
4. Die `@font-face`-Familiennamen werden korrigiert: `'Vend Sans'` → `VendSans`,
   `'DM Mono'` → `DMMono`, mit Verweis auf die selbst gehosteten woff2-Dateien.
   Heute greifen die Namen nicht, und die Sheets rendern still in `system-ui`.
5. Das CDN-`<script>` für Chart.js entfällt zugunsten von
   `src/assets/js/chart.umd.min.js`.
6. Das lokale Topnav wird durch die Topbar der App-Shell ersetzt.

## Noch nicht umgesetzt

| Sheet | Grund |
|---|---|
| `04-add-product.html` | Die App legt Produkte in einem Modal an. Eine eigene Seite wäre ein Feature, kein Design. |
| `17-product-detail.html` | Die App hat keine schreibgeschützte Produktansicht. Siehe `deviations.md` #2. |

Beide Sheets bleiben als Entwurf für den jeweiligen Feature-PR liegen.

## Nummerierung

Die Nummern 01–19 sind lückenlos, danach kommen 21 und 22. `20` existiert
nicht — es gab nie ein Screen 20. Die Lücke bleibt, damit eine Nummer nie
zweimal vergeben wird.

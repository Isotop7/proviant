# Spec-Sheets

21 eigenständige HTML-Dateien, die je einen Bildschirm des Design-Systems
zeigen. Sie sind **Referenz-Mockups**, kein Code und keine Testfälle — der
Beleg ist der Screenshot, nicht ein grüner Build.

## Stand der Migration

Die Sheets tragen **keine eigene Palette mehr**. Sie binden

- `../tokens.css` und `../tokens.dark.css` — die generierten Projektionen von
  `design-system/_tokens.scss` und `_tokens-dark.scss`,
- `../../src/assets/css/main.css` — das kompilierte App-Stylesheet mit
  Bootstrap, den Icons und den selbst gehosteten Fonts,
- `../../src/assets/js/chart.umd.min.js` — das vendorte Chart.js.

Damit kann ein Sheet nicht mehr von der App abweichen: die Farben, die
Schriften und die Icons kommen aus derselben Quelle. `task design-check`
prüft das ab Phase 6 — ein eigenes `:root` oder ein fehlender Token-Link
scheitert den Build.

## Was die Sheets nicht sind

Sie zeichnen das System, sie führen es nicht aus. Die Komponenten in ihren
`<style>`-Blöcken sind eine *Zeichnung* der Bootstrap-Komponenten mit den
kanonischen Tokens — der Button im Sheet ist nicht der Button der App, er
sieht nur so aus. Wer die echten Komponenten sehen will, fährt die App an.

Deshalb bleiben auch einzelne Farbliterale in den Sheets stehen: Sie gehören
zu den gezeichneten Komponenten, nicht zur Palette. Das ist in
`deviations.md` vermerkt.

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

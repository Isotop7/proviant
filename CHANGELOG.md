# Changelog

## [0.14.0] - 2026-05-14

### CI/CD
- Update git-cliff range syntax and simplify template

### Changed
- Update packages- Refactor(assets): cache DOM refs in products JS
Closes: #290

### Fixed
- Fix scss

### Miscellaneous
- Update go.sum and relax delta validation

## [0.13.0] - 2026-05-14

### Added
- Feat(api): add input validation for amount displayName household name
Closes: #291- Feat(router): add rate limiting for password and scan endpoints
Closes: #293

### Changed
- Update version- Refactor(router): add RequireHouseholdAdmin middleware
Closes: #289

### Fixed
- Fix scss- Fix test errors

### Miscellaneous
- Update CHANGELOG.md for v0.12.2

## [0.12.3] - 2026-05-14

### Changed
- Update version

## [0.12.2] - 2026-05-14

### Changed
- Update version

### Fixed
- Fix release mechanism

### Miscellaneous
- Update CHANGELOG.md for v0.12.1

## [0.12.1] - 2026-05-14

### Added
- **nav:** Add version badge

### CI/CD
- Create release on tag push

### Changed
- Update version

### Miscellaneous
- Update CHANGELOG.md for v0.12.0

## [0.12.0] - 2026-05-13

### Added
- Feat(webhooks): add context-aware webhook delivery with timeout
Closes: #282- Feat(api): add ProductService
Closes: #286- **api:** Add calendar export

### Changed
- Refactor(api): extract handler boilerplate into AppContext
Closes: #279- Change(api): clean up handlers
Closes: #280- Refactor(api): extract query types to queryparser
Closes: #281- Refactor(templates): split userSettings.tmpl into 4 partials
Closes: #284- Refactor(assets): merge JS modules into single proviant.js
Closes: #285- Refactor(assets): unify bulk action helper
Closes: #288- Update golangci-lint version

### Fixed
- **api:** Ensure baseURL trailing slash- Fix css- Fix(api): wrap invitation email in transaction
Closes: #283

### Miscellaneous
- Update CHANGELOG.md for v0.11.0- **assets:** Fix offline button to anchor tag- **templates:** Fix embed pattern

### Testing
- Add household assertions

## [0.11.0] - 2026-05-09

### Added
- Feat(audit): add audit logging
Closes: #180- Add demo instance

### Changed
- Fix golangci errors- Update packages and make vuln scan optional- Update README and docs

### Fixed
- Fix js file

### Miscellaneous
- Update CHANGELOG.md for v0.10.0- Chore: update auth, config, middleware, and frontend
Closes: #200

## [0.10.0] - 2026-05-07

### Added
- **api:** Unify error responses- **web:** Add skeleton loading and image fallbacks- **router:** Add request-id and user logging- **models:** Add db indexes- **assets:** Update user settings- **templates:** Update nav- **templates:** Update product card- **assets:** Improve products creation barcode handling- Feat(templates): add brand link
Closes: #241- Feat(templates): improve products UI
Closes: #242- Feat(templates): add household anchor
Closes: #243- Feat(templates): update page templates
Closes: #244- Add login migration prompt- Feat(web): update auth and settings
Closes: #246- Feat(api): add and rename product archive endpoints
Closes: #276- Add sonar exclusions
fix bash script
refactor dockerfile- Add comments- Add sonar export- Update product listing- **web:** Redesign auth page- Feat: add bulk consume/waste
Closes: #277- Feat: add trusted proxies
Closes: #332- Feat(products): track removal reason
Closes: #278- Feat: configurable upload size
Closes: #340- Feat(models): add configuration model and tests
Closes: #348- Feat: add debug mode support
Closes: #357- Feat: add rate limit config
Closes: #358- Feat(web): add sw update banner
Closes: #368- Feat(controllers): add Telegram poller pool
Closes: #366- Feat: add storage hint support for OpenFoodFacts
Closes: #386

### Changed
- **api:** Use uint for IDs- **api:** Introduce repository pattern- **assets:** Cache product images in sw and browser- **controllers:** Improve testability- Refactor(assets): refactor dashboard
Closes: #249- Refactor Signup function to use distinct helper functions
use const instead of duplicating strings
remove unused var- Refactor AdminResetUserPassword
use consts- Refactor building stats- **api:** Extract helpers- Extract magic strings- Use query constants- Use constants- **notification:** Simplify sender- Replace magic strings- Extract helpers- Extract helpers- Extract helpers- Extract types- **web:** Pass by pointer- Extract helpers- Extract helpers- Refactor(controllers): extract provider init helpers
Closes: #353- Refactor: rename error variables
Closes: #360

### Fixed
- **controllers:** Check repo- Fix scss warnings- Fix language code- Fix unicode char- Fix templating error- Fix js error and update docs- Fix(api): safe type assertion
Closes: #334- Fix(api): validate EAN-13
Closes: #346- Fix(api): correct HTTP status codes
Closes: #355- Fix: align log levels
Closes: #356- Fix(api): validate limit param
Closes: #364- Fix(router): batch revoked token cleanup
Closes: #365

### Miscellaneous
- Drop kilo lock- **web:** Update product pages- Style(templates): update button styles
Closes: #240- Style(assets): add blur validation
Closes: #245- Chore: add UI/UX standards
Closes: #250- Style(templates): update text hierarchy
Closes: #255- Style: update frontend
Closes: #253
Closes: #252- Style(templates): fix footer attribution text
Closes: #256- Style(templates): add household id help text
Closes: #247- Modernize scss- **login:** Drop backgrounds- **sonar:** Add duplications export

### Removed
- Remove unused function- Remove unused var- Delete temporary file- Remove login hints from login page

### Security
- Security: use CSP nonces
Closes: #330- Security(webhook): validate URLs
Closes: #338- Security: add CSRF protection
Closes: #336- Security: increase password policy defaults
Closes: #344

### Testing
- Add api and controller tests- **api:** Update onboarding

## [0.9.0-rc5] - 2026-04-25

## [0.9.0-rc4] - 2026-04-25

## [0.9.0-rc3] - 2026-04-25

## [0.9.0-rc2] - 2026-04-25

### Changed
- Update actions

## [0.9.0-rc1] - 2026-04-25

### Miscellaneous
- Update CHANGELOG.md for v0.9.0-rc5- Add recipe mobile screenshot

## [0.9.0] - 2026-04-24

### Added
- **streak:** Add household waste-free streak tracking- **products:** Add storage location support- Feat: add automated changelog generation with git-cliff and release
workflow- Enhance products page UI with mobile improvements and animations

### Changed
- Update icons and design system- Update design system- Update design- Update icons- Refactor code- Refactor(scss): standardize oklch color syntax and property ordering
Normalize oklch values to minimal form (e.g. .5 vs 0.50, add deg)
and reorder properties consistently across all rules for readability.
No functional changes.- Update design system

### Documentation
- Update documentation

### Fixed
- Fix nilness check

### Miscellaneous
- Update logo and stylesheet

### Removed
- Drop commit tools

## [0.8.0] - 2026-04-20

### Added
- Feat(notifications): add monthly waste report
Enable households to receive automated monthly emails with waste
statistics
to track and reduce food waste. Reports include total wasted items,
waste rate,
and comparison to previous month, sent on configured day/hour (UTC) to
opted-in members.- **settings:** Refactor notification settings layout- **notifications:** Add Telegram bot notification provider- **api:** Add Home Assistant integration endpoints- **telegram:** Move bot token to user scope

### Changed
- Update go to 1.25.9

### Fixed
- **lint:** Resolve golangci-lint and htmlhint violations

## [0.7.0] - 2026-04-17

### Added
- **products:** Add expiry urgency indicators and default sort by expiry- Feat: add display names and profile step to onboarding
Introduces user display names separate from login usernames for better
privacy.
Adds profile configuration as first onboarding step before household
selection.

### Changed
- Update default csp- Refactor: reduce code duplication in API and frontend
Consolidates ID parsing, error handling, and bulk operations into shared
helpers.
Improves code maintainability and consistency across product and
household APIs.- **api/v1:** Extract Gin context helpers to cut handler boilerplate

## [0.6.1] - 2026-04-15

## [0.5.0] - 2026-04-10

## [0.4.0] - 2026-04-04

## [0.3.0] - 2025-12-26

### Added
- Implement client side search- Add ci- Add soft delete update docs- Add optional soft deletion on api call- Add zed debugging config- Add archiving on frontend- Add methods and frontend to get archived products update best before
date icon- Add deletion to archive get unscoped products when checking for deletion- Add deletion to archive get unscoped products when checking for deletion- Add golangci.yml and use action- Add api model and partial- Add motion effect on click make card clickable- Add template- Implement option bar and card selection in archive page add API and
database functions for bulk restore update docs- Add hero-icon class- Add basic font and fix imports- Add const strings cleanup switch statements that could be ifs- Add flagReplace function- Add all country codes- Add changelog- Add interface and unit test- Add unit tests- Add code coverage make target- Add code coverage to ci- Add unit test- Add logger check from context- Add unit tests- Add unit test- Add unit test- Add default values- Add test unit- Add unit test- Add unit tests- Add unit test- Add interface- Add safeguard for empty limit add unit test- Add new tiles reformat code- Implement ntfy
add notification controller implementation for ntfy
add UI and backend features to set notification config
move fromAddress to smtp config
reformat code- Add quick pickers- Add animation- Add household functions in frontend, backend and database- Add guard for invalid timestamps- Add design implementation guide- Add vulnerability scans
update mods- Feat(api): add personal access tokens for headless API auth
Users create named tokens stored as SHA-256 hashes. Tokens use
Authorization: Bearer header with proviant_pat_ prefix. Middleware
checks PAT before falling back to JWT. Token value shown once at
creation.
Closes #201- Feat(api): add CSV/JSON bulk export endpoints
Add GET
/api/v1/products/export/{products.csv,products.json,archive.csv,full.json}
with optional date range filtering and 1 req/min rate limit per user.
Closes #203- Feat(calendar): add iCal export for product expiry dates
Implement GET /api/v1/calendar/export.ics with token query param auth
for calendar app subscription (Google Calendar, Apple Calendar,
Thunderbird).
Include POST/DELETE/GET /api/v1/calendar/token for token management.
Refs #193- Add frontend controls for calendar sync

### Changed
- Update gitlab ci- Update makefile and image- Update filtering and search
add label for mobile menu- Update packages- Update docs- Update job name- Update docs- Update image version- Update layout- Update button order- Update golanci-lint update docs remove old gitlab ci files- Update go modules and node modules- Update docs and README- Update styling- Update modules- Update font to VendSans

### Documentation
- Add swagger documentation to api and web handlers

### Fix
- Readd frontend handler to archive products

### Fixed
- Fix margin on mobile view- Fix location of warning- Fix docker and makefile- Fix file name- Fix uncatched error in defer function- Fix unauthenticated error switch back to POST- Fix folder name- Fix comment- Fix path update version- Fix var name- Fix registry url- Fix selecting and deselecting card update empty archive page- Fix passing of OpenFoodFactsAPI controller- Fix search redirect and duplicated html element- Fix typo- Fix tests- Fix saving notification preferences- Fix empty return of array- Fix js lint warnings

### Removed
- Remove debug step- Remove unneccessary string format use empty string comparison pass some
objects by reference fix shadow import- Remove duplicated try- Remove duplicated try- Remove search site- Remove tooltips and normalize font size of input box

## [0.2.0] - 2024-11-30

### Added
- Add comment- Add title tag
update docker image user
use constant for API responses
fix return of bool
update go mod- Add route to scan barcode
add cors middleware
add cors config- Add target
update dockerbuild paths- Add template cache and custom write function
add blank handlers and pages
fix css and add popper- Add blank files and handlers
add template cache- Implement redirect to auth page
allow custom base for Render()
add auth page
add frontend object for request handling- Add cookie transparentl
send cookie to client to limit client-side-scripting
cleanup forms
add boostrap js files- Add logout function and button
update css
update layout and navbar- Add script to create products
update template
add bootstrap-icons- Add fonts and update css
add footer
use light theme- Add scripts for scanning qr codes- Add loading indicators- Add expiro lib
add product view- Add template files and handlers- Add error check- Add class for middlewars
pass Authorizator and UnauthorizedFunc as parameters
use user aware processing on frontend routes
add custom error page
add errors and rewrite APIResponses- Add user settings template
add custom middleware- Add basic validation to login data
add error box to login data
fix inputUsername border- Add signup and alerts to auth page- Add additional methods to check user uniqueness
add new custom errors and cast from error
allow short usernames- Add success toast
update api call- Add product view and helper functions- Add comments and refactor code- Add functions to update user and password
new api handlers
frontend wiring and scripts
add option for unauthenticated smtp call- Add validation checks- Add default value for expireAt
ref #49- Add tile
add database function to generate tile stats
product: add click handler table row
product: add notifiy timestamp
update ui style- Add loading spinner
update UI- Add product edit page layout
refactor product view
reorder nav- Add product edit page
add edit product function
add patch dto- Add trim- Add basic search- Add sqlite backend- Add sqlite compose file
update dockerfile for sqlite deployment- Add html5-qrcode for live scanning
cleanup UI and code- Add API call to get user products by barcode- Add instance query and button toolbar- Add fading and clear alert on new create- Add modal and implement deletion- Add new icons
add favicon
update footer and auth site- Add icons- Add screenshots- Add household functionality
add database migrations
update functions to be scoped on household
add template for no products
update frontend

### Changed
- Refactor code- Update doc- Update doc- Refactor code
add logging
update response types
update docs- Update README- Refactor code- Update README- Update CI- Update README- Update README- Update gitignore and fix ci change path- Update README- Update ci- Update comment- Update README- Update DOCKERFILE- Update doc- Update css steps and add makefile target- Update theme- Refactor product scan page- Update signup to include mail- Refactor code- Update doc- Update modules
add init step for go modules- Refactor js code- Update offcanvas navbar- Update ui
increase font size
dont use card on view
fix checkbox trigger function- Update layout of create page- Update search page
update humanDate templating function- Update product view page and add edit button- Update README- Update README
update docs
update go modules- Update README- Update css
update fonts
update node modules- Update dockerfile
update alpine and golang
build and include assets
use default sqlite template- Update logos- Refactor dockerfile- Update go modules- Update lint image- Update images and README- Update README- Update tile layout- Update home template
hide footer for now- Update search page and portal page layout- Update main menu- Update navigation- Update user settings- Update css
fix script name- Update fonts- Update site- Update product cards
update navbar
make qr code scanner responsive- Update background to border- Update packages- Update go modules- Refactor code
add handler to show all products for barcode- Update icons- Update gitignore

### Fixed
- Fix ineffectual assignment- Fix init target- Fix redirect state- Fix login screen contrast
fix username input border- Fix comment- Fix setting timestamps- Fix null redirect- Fix mail address check and user id query
update css- Fix rewriting of headers- Fix passing of json and add break in switch
add doc and screenshots- Fix badgifyCategories when not splittable
add splitString templating function- Fix duplicated import
fix invalid variable name- Fix dereferencing

### Miscellaneous
- Style updates

### Removed
- Remove padding and rounding on image- Remove scan sites and update create pages

## [0.1] - 2023-12-29

### Added
- Add basic web ui layout- Add highlighting for past best before date- Add time limit to regex operation- Implement quick returns if Notification is disabled and embedded Broker is used- Add database creation step- Add platformio build 0.1.0
supports reading config
supports setting config (not yet used in main.cpp)
supports reading serial in from scanner module- Add solution file- Add search bar and page- Add dockerfile and build jobs- Add remote url- Add handler for unknown topic- Add barcode to error message- Add debug steps- Add debug step- Add openfoodfactsapicontroller
populate object with data- Add distinct api method to set ExpireAt
rename BestBefore to ExpireAt
add filter to api call- Add gitlab ci- Add mail notification
add mail template
change toAddress to []string- Add authenticaton middleware
add database controller
refactor model structure

### Changed
- Update config check and values- Update namespace definition- Update workflow- Update login step- Update ci tasks- Update ci steps- Update ci- Update CI- Update build file- Update path- Update go version- Update actions
add template- Update dockerfile- Update context- Update actions- Update CRUD methods- Update name- Update image- Update build target- Update stage- Update notifiedAt after notification is sent- Update mail template- Update var for privat push- Update README- Update README
relocate file- Update README- Update README- Update README- Update README- Update README

### Fixed
- Fix location of sln copy- Fix build- Fix typo- Fix comments
fix names
update ci- Fix version- Fix naming- Fix push uri- Fix docker build- Fix ci- Fix annotations
fix naming- Fix ci

### Removed
- Remove vscode- Remove old folder- Remove context- Remove step- Remove exists rule- Remove html code- Remove artifacts

<!-- generated by git-cliff -->

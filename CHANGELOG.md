# Changelog

## [0.20.0] - 2026-10-06

### Added
- (f1a5f776): Configurable server bind host
- (e0a277b5): Add CSV product import
- (afa6f2f8): Feat(products): add batch scan mode
Closes: #59
- (03ee096e): Feat(products): add batch scan mode
Closes: #59
- (ba9e2a4a): Feat: add receipt scan and bulk product import
Closes: #61
- (f270710b): Add taskfile tasks for ui-capture
- (37610e98): **web:** Add dark mode


### CI/CD
- (4efd3a6f): **release:** Publish docker images to ghcr


### Documentation
- (dfcbed56): Regenerate swagger and godoc


### Fixed
- (7e05f39b): Expiry badge and header overflow
- (7009a387): **lint:** Resolve golangci-lint findings


### Miscellaneous
- (8401ff14): Update CHANGELOG.md for v0.19.0
- (7410a50b): Expand demo seed and screenshots
- (0d14f3da): Add ui-capture skill
- (83f7c950): **ui-capture:** Update capture skill
- (a3135eac): **templates:** Replace inline styles with utility classes


## [0.19.0] - 2026-10-04

### Added
- (39e14116): Add design guidelines from open-design
- (793fe2f3): Add ui refresh plan
- (980ef9a1): Feat(products): track opened product shelf life
Closes: #300
- (88142d84): **notifications:** Show effective expiry date
- (aeb9b281): **products:** Add cook action for partial consumption
- (60fe069a): Add caching and use node 24
- (2f3d84c0): Feat(controllers): add Mealie and Tandoor recipe providers
Closes: #47


### CI/CD
- (5525d73e): Centralize npm audit gate in shared script


### Changed
- (95bc162e): Update design
- (6d835e38): Update modules
- (da2d754a): Update go version


### Fixed
- (3f3473ff): Fix github actions
- (19bfa5dc): Satisfy staticcheck in VAPID key gen and cook error switch


### Miscellaneous
- (20b0fd2e): **deps:** Bump go 1.26.0 and update dependencies
- (25af02a6): Update CHANGELOG.md for v0.17.0
- (8d02ab66): Merge milestone/0.18.0 into milestone/0.19.0
- (ad849437): Bump golangci-lint to v2.14.0


## [0.17.0] - 2026-06-14

### Added
- (7a44fe88): Add password reset flow and frontend
- (8b17dfd6): Feat(stats): add waste analytics
Closes: #312
- (f56c0869): Feat(consumption): add restock suggestion service
Closes: #313
- (86b6abc8): Feat(notifications): notify household on streak reset
Closes: #314
- (e92223a8): Feat(stats): add 12months, sort, limit, top products
Closes: #315


### Changed
- (3b84b6f4): Update docs
- (94e60364): Update packages


### Fixed
- (eb7b0256): Fix mail digest frequency
- (adb1351e): Fix css and golangci-lint


### Miscellaneous
- (c622a464): Update CHANGELOG.md for v0.16.0


## [0.16.0] - 2026-05-17

### Added
- (0c5c8778): Feat(shopping): add household shopping list
Closes: #308
- (db547c0a): Feat(shopping): add household shopping list
Closes: #309
- (5ad8e404): **products:** Add actions column to list view
- (a76b4132): Feat(household): add member role management
Closes: #310
- (0b926654): Feat(products): add per-product privacy
Closes: #311


### Fixed
- (7ec708dc): **assets:** Repair product selection and checkbox handling


### Miscellaneous
- (4ad75a58): Update CHANGELOG.md for v0.15.0
- (05233ef0): Update CHANGELOG.md for v0.15.0
- (eadae823): **templates:** Use hyphen instead of unicode minus


## [0.15.0] - 2026-05-15

### Added
- (3216658f): Feat(api): add calendar token expiry and rotation
Closes: #296
- (3ffa3e4b): Feat(products): per-product expire lead time
Closes: #297
- (1ffa18ae): Feat(notifications): add web push notifications
closes: #298
- (0f3eb20b): Feat(notifications): add expiry mail digest
Closes: #299
- (fe1760d3): Feat: add auto shopping list
Closes: #307
- (7b8ee409): Add debug step


### Changed
- (bd9ecca5): Update release mechanism
- (0df85b21): Update cliff config
- (e7dfbb69): Update release mechanism


### Fixed
- (c322b7f7): Fix git-cliff install step
- (f646df54): Fix tag finding logic


### Miscellaneous
- (cdaa15d0): Update CHANGELOG.md for v0.14.0
- (2f9f2c9a): Update CHANGELOG.md for v0.15.0
- (b4bcf453): Update CHANGELOG.md for v0.15.0
- (cffa4620): Update CHANGELOG.md for v0.15.0


## [0.14.0] - 2026-05-14

### CI/CD
- (cbe5c7bb): Update git-cliff range syntax and simplify template


### Changed
- (2bf78299): Update packages
- (131db738): Refactor(assets): cache DOM refs in products JS
Closes: #290


### Fixed
- (d26a0cf9): Fix scss


### Miscellaneous
- (dd94e471): Update go.sum and relax delta validation


## [0.13.0] - 2026-05-14

### Added
- (e6f84aa5): Feat(api): add input validation for amount displayName household name
Closes: #291
- (3ddee155): Feat(router): add rate limiting for password and scan endpoints
Closes: #293


### Changed
- (3e343bef): Update version
- (36c02ec0): Refactor(router): add RequireHouseholdAdmin middleware
Closes: #289


### Fixed
- (60875e6e): Fix scss
- (8c47b0dc): Fix test errors


### Miscellaneous
- (04c8de32): Update CHANGELOG.md for v0.12.2


## [0.12.3] - 2026-05-14

### Changed
- (64beb748): Update version


## [0.12.2] - 2026-05-14

### Changed
- (69e351d2): Update version


### Fixed
- (a9ad2f3b): Fix release mechanism


### Miscellaneous
- (3a98c22e): Update CHANGELOG.md for v0.12.1


## [0.12.1] - 2026-05-14

### Added
- (d5145403): **nav:** Add version badge


### CI/CD
- (ed5939e9): Create release on tag push


### Changed
- (c727260f): Update version


### Miscellaneous
- (b15228e5): Update CHANGELOG.md for v0.12.0


## [0.12.0] - 2026-05-13

### Added
- (d61ddf01): Feat(webhooks): add context-aware webhook delivery with timeout
Closes: #282
- (78759c66): Feat(api): add ProductService
Closes: #286
- (24ac30f7): **api:** Add calendar export


### Changed
- (7daf8feb): Refactor(api): extract handler boilerplate into AppContext
Closes: #279
- (91e49724): Change(api): clean up handlers
Closes: #280
- (0405c7ed): Refactor(api): extract query types to queryparser
Closes: #281
- (4cd515c4): Refactor(templates): split userSettings.tmpl into 4 partials
Closes: #284
- (94447306): Refactor(assets): merge JS modules into single proviant.js
Closes: #285
- (83361374): Refactor(assets): unify bulk action helper
Closes: #288
- (3a93ccad): Update golangci-lint version


### Fixed
- (51d5f3c3): **api:** Ensure baseURL trailing slash
- (0bf99739): Fix css
- (cd5b9285): Fix(api): wrap invitation email in transaction
Closes: #283


### Miscellaneous
- (300fd6fc): Update CHANGELOG.md for v0.11.0
- (85233084): **assets:** Fix offline button to anchor tag
- (5be235f6): **templates:** Fix embed pattern


### Testing
- (8dbdd41c): Add household assertions


## [0.11.0] - 2026-05-09

### Added
- (e80b5132): Feat(audit): add audit logging
Closes: #180
- (fc4c1220): Add demo instance


### Changed
- (284ec9ec): Fix golangci errors
- (494ae850): Update packages and make vuln scan optional
- (c68a5d94): Update README and docs


### Fixed
- (56226fe0): Fix js file


### Miscellaneous
- (df612b3b): Update CHANGELOG.md for v0.10.0
- (52419c96): Chore: update auth, config, middleware, and frontend
Closes: #200


## [0.10.0] - 2026-05-07

### Added
- (a39550cd): **api:** Unify error responses
- (d7fb2e86): **web:** Add skeleton loading and image fallbacks
- (f25b4e80): **router:** Add request-id and user logging
- (3bbc8dc6): **models:** Add db indexes
- (65f094fe): **assets:** Update user settings
- (dde8a50a): **templates:** Update nav
- (87b881e1): **templates:** Update product card
- (53125d37): **assets:** Improve products creation barcode handling
- (9bf14e4c): Feat(templates): add brand link
Closes: #241
- (0c66a365): Feat(templates): improve products UI
Closes: #242
- (17b167b7): Feat(templates): add household anchor
Closes: #243
- (a73e7027): Feat(templates): update page templates
Closes: #244
- (9b2a3a34): Add login migration prompt
- (1dab2fb1): Feat(web): update auth and settings
Closes: #246
- (7ac9a118): Feat(api): add and rename product archive endpoints
Closes: #276
- (b333423b): Add sonar exclusions
fix bash script
refactor dockerfile
- (9c4adeb9): Add comments
- (7040cecb): Add sonar export
- (1115660b): Update product listing
- (4dcfe807): **web:** Redesign auth page
- (b7606313): Feat: add bulk consume/waste
Closes: #277
- (77a9bee5): Feat: add trusted proxies
Closes: #332
- (209014fb): Feat(products): track removal reason
Closes: #278
- (7f6074e9): Feat: configurable upload size
Closes: #340
- (4d9104d7): Feat(models): add configuration model and tests
Closes: #348
- (2264e737): Feat: add debug mode support
Closes: #357
- (a02db823): Feat: add rate limit config
Closes: #358
- (69d1278b): Feat(web): add sw update banner
Closes: #368
- (0887e78d): Feat(controllers): add Telegram poller pool
Closes: #366
- (b4d96079): Feat: add storage hint support for OpenFoodFacts
Closes: #386


### Changed
- (c5e190b1): **api:** Use uint for IDs
- (5302ab6c): **api:** Introduce repository pattern
- (b010d261): **assets:** Cache product images in sw and browser
- (e4fccaf3): **controllers:** Improve testability
- (18cde972): Refactor(assets): refactor dashboard
Closes: #249
- (0d0a43ba): Refactor Signup function to use distinct helper functions
use const instead of duplicating strings
remove unused var
- (c25854e4): Refactor AdminResetUserPassword
use consts
- (cea76aca): Refactor building stats
- (eb943bbb): **api:** Extract helpers
- (dcbd6b78): Extract magic strings
- (a947b943): Use query constants
- (06ec3a5f): Use constants
- (2bb32a43): **notification:** Simplify sender
- (06d80015): Replace magic strings
- (21da94de): Extract helpers
- (ffa0bc15): Extract helpers
- (5f4c3ee7): Extract helpers
- (e6ea409d): Extract types
- (48d3e318): **web:** Pass by pointer
- (e2912e25): Extract helpers
- (cc37171f): Extract helpers
- (f4f4df6a): Refactor(controllers): extract provider init helpers
Closes: #353
- (7611afc3): Refactor: rename error variables
Closes: #360


### Fixed
- (f335c243): **controllers:** Check repo
- (8fcf7dbf): Fix scss warnings
- (9c4b34f2): Fix language code
- (0867b6ff): Fix unicode char
- (3ef025bd): Fix templating error
- (3741ded8): Fix js error and update docs
- (9b7fae65): Fix(api): safe type assertion
Closes: #334
- (4135f0c6): Fix(api): validate EAN-13
Closes: #346
- (967afadd): Fix(api): correct HTTP status codes
Closes: #355
- (cc2ab29a): Fix: align log levels
Closes: #356
- (0d89cf31): Fix(api): validate limit param
Closes: #364
- (c55e2e72): Fix(router): batch revoked token cleanup
Closes: #365


### Miscellaneous
- (a2c168ea): Update CHANGELOG.md for v0.9.0-rc5
- (254b583b): Drop kilo lock
- (2c5d1bdf): **web:** Update product pages
- (2562ea3e): Style(templates): update button styles
Closes: #240
- (11de45b7): Style(assets): add blur validation
Closes: #245
- (7809ebbc): Chore: add UI/UX standards
Closes: #250
- (c58bc6a4): Style(templates): update text hierarchy
Closes: #255
- (cf11e47e): Style: update frontend
Closes: #253
Closes: #252
- (afae97ff): Style(templates): fix footer attribution text
Closes: #256
- (6ecc727a): Style(templates): add household id help text
Closes: #247
- (7dd37de5): Modernize scss
- (747129f5): **login:** Drop backgrounds
- (5a807c4e): **sonar:** Add duplications export


### Removed
- (6b0c6925): Remove unused function
- (a6b95d81): Remove unused var
- (75618952): Delete temporary file
- (4590d974): Remove login hints from login page


### Security
- (7424f0c8): Security: use CSP nonces
Closes: #330
- (e6b9a914): Security(webhook): validate URLs
Closes: #338
- (7b7978e0): Security: add CSRF protection
Closes: #336
- (7168546a): Security: increase password policy defaults
Closes: #344


### Testing
- (43b421e6): Add api and controller tests
- (d5a92297): **api:** Update onboarding


## [0.9.0-rc5] - 2026-04-25

## [0.9.0-rc4] - 2026-04-25

## [0.9.0-rc3] - 2026-04-25

## [0.9.0-rc2] - 2026-04-25

### Changed
- (b231f382): Update actions


## [0.9.0-rc1] - 2026-04-25

### Miscellaneous
- (36588c1c): Add recipe mobile screenshot


## [0.9.0] - 2026-04-24

### Added
- (bb2d1d43): **streak:** Add household waste-free streak tracking
- (8d985656): **products:** Add storage location support
- (163732de): Feat: add automated changelog generation with git-cliff and release
workflow
- (caed8313): Enhance products page UI with mobile improvements and animations


### Changed
- (b5e4628b): Update icons and design system
- (51e141f1): Update design system
- (b9380a10): Update design
- (00078b04): Update icons
- (4e5f7bb9): Refactor code
- (806113ce): Refactor(scss): standardize oklch color syntax and property ordering
Normalize oklch values to minimal form (e.g. .5 vs 0.50, add deg)
and reorder properties consistently across all rules for readability.
No functional changes.
- (50f7b237): Update design system


### Documentation
- (55821873): Update documentation


### Fixed
- (acd187f9): Fix nilness check


### Miscellaneous
- (6b4d71a9): Update logo and stylesheet


### Removed
- (540b62b6): Drop commit tools


## [0.8.0] - 2026-04-20

### Added
- (51559bd3): Feat(notifications): add monthly waste report
Enable households to receive automated monthly emails with waste
statistics
to track and reduce food waste. Reports include total wasted items,
waste rate,
and comparison to previous month, sent on configured day/hour (UTC) to
opted-in members.
- (4d7e041f): **settings:** Refactor notification settings layout
- (ec9c3e1f): **notifications:** Add Telegram bot notification provider
- (1d28d520): **api:** Add Home Assistant integration endpoints
- (9165f1be): **telegram:** Move bot token to user scope


### Changed
- (7c9179a7): Update go to 1.25.9


### Fixed
- (84030c3e): **lint:** Resolve golangci-lint and htmlhint violations


## [0.7.0] - 2026-04-17

### Added
- (9424dd4d): **products:** Add expiry urgency indicators and default sort by expiry
- (077a5149): Feat: add display names and profile step to onboarding
Introduces user display names separate from login usernames for better
privacy.
Adds profile configuration as first onboarding step before household
selection.


### Changed
- (4becc4f6): Update default csp
- (64f4ad3c): Refactor: reduce code duplication in API and frontend
Consolidates ID parsing, error handling, and bulk operations into shared
helpers.
Improves code maintainability and consistency across product and
household APIs.
- (a67a98de): **api/v1:** Extract Gin context helpers to cut handler boilerplate


## [0.6.1] - 2026-04-15

### Added
- (20499dd7): Feat(api): add personal access tokens for headless API auth
Users create named tokens stored as SHA-256 hashes. Tokens use
Authorization: Bearer header with proviant_pat_ prefix. Middleware
checks PAT before falling back to JWT. Token value shown once at
creation.
Closes #201
- (bcc5211d): Feat(api): add CSV/JSON bulk export endpoints
Add GET
/api/v1/products/export/{products.csv,products.json,archive.csv,full.json}
with optional date range filtering and 1 req/min rate limit per user.
Closes #203
- (78702508): Feat(calendar): add iCal export for product expiry dates
Implement GET /api/v1/calendar/export.ics with token query param auth
for calendar app subscription (Google Calendar, Apple Calendar,
Thunderbird).
Include POST/DELETE/GET /api/v1/calendar/token for token management.
Refs #193
- (ae0a2512): Add frontend controls for calendar sync


### Documentation
- (7b49253f): Add swagger documentation to api and web handlers


## [0.5.0] - 2026-04-10

### Added
- (3fb9b6aa): Add vulnerability scans
update mods


### Changed
- (445c3e72): Update font to VendSans


### Fixed
- (18278489): Fix js lint warnings


## [0.4.0] - 2026-04-04

### Added
- (a3f9e59e): Add changelog
- (85553ea4): Add interface and unit test
- (3ce6e422): Add unit tests
- (174fa4ad): Add code coverage make target
- (4c025fc2): Add code coverage to ci
- (0ca15641): Add unit test
- (0317b829): Add logger check from context
- (ad9e875c): Add unit tests
- (fe665125): Add unit test
- (82aec25c): Add unit test
- (26bc9f76): Add default values
- (5fd46a9f): Add test unit
- (a9bca6b7): Add unit test
- (7988b462): Add unit tests
- (7dbc974a): Add unit test
- (9193b613): Add interface
- (9bd245e4): Add safeguard for empty limit add unit test
- (ac700fe6): Add new tiles reformat code
- (46bb252e): Implement ntfy
add notification controller implementation for ntfy
add UI and backend features to set notification config
move fromAddress to smtp config
reformat code
- (a3bfc458): Add quick pickers
- (18ebfe42): Add animation
- (2c48d08e): Add household functions in frontend, backend and database
- (1a4eebcd): Add guard for invalid timestamps
- (a5dbc453): Add design implementation guide


### Changed
- (4c8b00bd): Update go modules and node modules
- (add31e6a): Update docs and README
- (8564c99c): Update styling
- (10011f43): Update modules


### Fix
- (ddbec80c): Readd frontend handler to archive products


### Fixed
- (65313f63): Fix typo
- (fe81536e): Fix tests
- (e19f5adb): Fix saving notification preferences
- (723ac787): Fix empty return of array


### Removed
- (e8ba5a40): Remove tooltips and normalize font size of input box


## [0.3.0] - 2025-12-26

### Added
- (9dd62232): Implement client side search
- (c81ae763): Add ci
- (8e5134e7): Add soft delete update docs
- (cefb26fd): Add optional soft deletion on api call
- (67f06922): Add zed debugging config
- (38159c75): Add archiving on frontend
- (7467768a): Add methods and frontend to get archived products update best before
date icon
- (d4d937bf): Add deletion to archive get unscoped products when checking for deletion
- (5773bdb6): Add deletion to archive get unscoped products when checking for deletion
- (80d36f8d): Add golangci.yml and use action
- (b28df158): Add api model and partial
- (84944699): Add motion effect on click make card clickable
- (971f45ae): Add template
- (8fb16b26): Implement option bar and card selection in archive page add API and
database functions for bulk restore update docs
- (469f8bc8): Add hero-icon class
- (fabc9075): Add basic font and fix imports
- (f2f58235): Add const strings cleanup switch statements that could be ifs
- (590512a8): Add flagReplace function
- (d5bd5b83): Add all country codes


### Changed
- (d7acddf8): Update gitlab ci
- (76f9779a): Update makefile and image
- (accafe94): Update filtering and search
add label for mobile menu
- (7e0bd380): Update packages
- (33f2896a): Update docs
- (afb2b5ee): Update job name
- (c68789da): Update docs
- (fe985a09): Update image version
- (0086fce7): Update layout
- (225c86f6): Update button order
- (c6484d09): Update golanci-lint update docs remove old gitlab ci files


### Fixed
- (40ebf7a8): Fix margin on mobile view
- (fd937806): Fix location of warning
- (24bffcbc): Fix docker and makefile
- (32a29161): Fix file name
- (6cc796e0): Fix uncatched error in defer function
- (dccd076f): Fix unauthenticated error switch back to POST
- (15791741): Fix folder name
- (c80f82f3): Fix comment
- (163883ff): Fix path update version
- (546363eb): Fix var name
- (c6b790ac): Fix registry url
- (97a08787): Fix selecting and deselecting card update empty archive page
- (408df047): Fix passing of OpenFoodFactsAPI controller
- (7b2f4948): Fix search redirect and duplicated html element


### Removed
- (2b914fde): Remove debug step
- (90e15c5b): Remove unneccessary string format use empty string comparison pass some
objects by reference fix shadow import
- (c9158b74): Remove duplicated try
- (980d674c): Remove duplicated try
- (406c9f16): Remove search site


## [0.2.0] - 2024-11-30

### Added
- (c8990e7c): Add comment
- (79f15e44): Add title tag
update docker image user
use constant for API responses
fix return of bool
update go mod
- (73a4dbdc): Add route to scan barcode
add cors middleware
add cors config
- (550014bb): Add target
update dockerbuild paths
- (7bcee7bf): Add template cache and custom write function
add blank handlers and pages
fix css and add popper
- (4d3ca6b4): Add blank files and handlers
add template cache
- (9e4858de): Implement redirect to auth page
allow custom base for Render()
add auth page
add frontend object for request handling
- (a990c721): Add cookie transparentl
send cookie to client to limit client-side-scripting
cleanup forms
add boostrap js files
- (6d9e1954): Add logout function and button
update css
update layout and navbar
- (f153e423): Add script to create products
update template
add bootstrap-icons
- (69d0bbd0): Add fonts and update css
add footer
use light theme
- (a718b122): Add scripts for scanning qr codes
- (aaeca536): Add loading indicators
- (916c2886): Add expiro lib
add product view
- (1d66183b): Add template files and handlers
- (6a087a7a): Add error check
- (b19cdafb): Add class for middlewars
pass Authorizator and UnauthorizedFunc as parameters
use user aware processing on frontend routes
add custom error page
add errors and rewrite APIResponses
- (60d35b21): Add user settings template
add custom middleware
- (0afab598): Add basic validation to login data
add error box to login data
fix inputUsername border
- (b3668c8b): Add signup and alerts to auth page
- (154ab6f7): Add additional methods to check user uniqueness
add new custom errors and cast from error
allow short usernames
- (1d36cc98): Add success toast
update api call
- (1ded5581): Add product view and helper functions
- (c9b2a229): Add comments and refactor code
- (5f89e40d): Add functions to update user and password
new api handlers
frontend wiring and scripts
add option for unauthenticated smtp call
- (b1d8bf9e): Add validation checks
- (4c1fb10c): Add default value for expireAt
ref #49
- (e0a18c9e): Add tile
add database function to generate tile stats
product: add click handler table row
product: add notifiy timestamp
update ui style
- (55182db4): Add loading spinner
update UI
- (3968578b): Add product edit page layout
refactor product view
reorder nav
- (49923d66): Add product edit page
add edit product function
add patch dto
- (c5bf0419): Add trim
- (4c484333): Add basic search
- (b72d76db): Add sqlite backend
- (1b25d87a): Add sqlite compose file
update dockerfile for sqlite deployment
- (2683b4b9): Add html5-qrcode for live scanning
cleanup UI and code
- (d05b126a): Add API call to get user products by barcode
- (d056c69d): Add instance query and button toolbar
- (9cfc38c5): Add fading and clear alert on new create
- (d54ab55d): Add modal and implement deletion
- (f7358906): Add new icons
add favicon
update footer and auth site
- (8dec3ef2): Add icons
- (d1030954): Add screenshots
- (cd54e35d): Add household functionality
add database migrations
update functions to be scoped on household
add template for no products
update frontend


### Changed
- (55378137): Refactor code
- (2ed572e4): Update doc
- (280dea06): Update doc
- (2257a7a9): Refactor code
add logging
update response types
update docs
- (6ca489fe): Update README
- (d7faac90): Refactor code
- (2e28ec95): Update README
- (65f8aab2): Update CI
- (9655472e): Update README
- (ba3f4648): Update README
- (135bc92f): Update gitignore and fix ci change path
- (9d41c86f): Update README
- (de04a97c): Update ci
- (e9fe586f): Update comment
- (3510edc4): Update README
- (a2f7ed1a): Update DOCKERFILE
- (e506f3de): Update doc
- (1d096969): Update css steps and add makefile target
- (433e07ea): Update theme
- (9824301c): Refactor product scan page
- (17911f7b): Update signup to include mail
- (b963099c): Refactor code
- (fa7336eb): Update doc
- (0fc3cbb9): Update modules
add init step for go modules
- (5deae6b5): Refactor js code
- (a6f5e5ab): Update offcanvas navbar
- (2f4aa447): Update ui
increase font size
dont use card on view
fix checkbox trigger function
- (451ed48f): Update layout of create page
- (99862915): Update search page
update humanDate templating function
- (1d76adff): Update product view page and add edit button
- (0b0d092e): Update README
- (532a7859): Update README
update docs
update go modules
- (c740ca70): Update README
- (1e2f845e): Update css
update fonts
update node modules
- (42889313): Update dockerfile
update alpine and golang
build and include assets
use default sqlite template
- (ef6ac63a): Update logos
- (e1df0b43): Refactor dockerfile
- (1768b1a5): Update go modules
- (2291f7b1): Update lint image
- (12361a26): Update images and README
- (1e760476): Update README
- (db4fe7a7): Update tile layout
- (11215186): Update home template
hide footer for now
- (028816ef): Update search page and portal page layout
- (05e8078e): Update main menu
- (b4cdd31a): Update navigation
- (578c8682): Update user settings
- (660ef0fe): Update css
fix script name
- (652e5e62): Update fonts
- (487b9636): Update site
- (b5214a07): Update product cards
update navbar
make qr code scanner responsive
- (b08414e0): Update background to border
- (d4310365): Update packages
- (86d9f4ae): Update go modules
- (4bb5f4c0): Refactor code
add handler to show all products for barcode
- (f4220577): Update icons
- (5fef3ec6): Update gitignore


### Fixed
- (dea3e119): Fix ineffectual assignment
- (2a168c1a): Fix init target
- (f44d6bca): Fix redirect state
- (cc2a736f): Fix login screen contrast
fix username input border
- (9b2d6f77): Fix comment
- (92388146): Fix setting timestamps
- (d73164a6): Fix null redirect
- (e94ce068): Fix mail address check and user id query
update css
- (f949baf0): Fix rewriting of headers
- (de18d773): Fix passing of json and add break in switch
add doc and screenshots
- (a4a9e4fb): Fix badgifyCategories when not splittable
add splitString templating function
- (cbaac264): Fix duplicated import
fix invalid variable name
- (1b713a5f): Fix dereferencing


### Miscellaneous
- (1c7d60cb): Style updates


### Removed
- (28abd2df): Remove padding and rounding on image
- (ef9e0a31): Remove scan sites and update create pages


## [0.1] - 2023-12-29

### Added
- (c32aa257): Add basic web ui layout
- (9e3b75b9): Add highlighting for past best before date
- (a19018f8): Add time limit to regex operation
- (795e7315): Implement quick returns if Notification is disabled and embedded Broker is used
- (4c72cd44): Add database creation step
- (19499a4a): Add platformio build 0.1.0
supports reading config
supports setting config (not yet used in main.cpp)
supports reading serial in from scanner module
- (f0c4b95b): Add solution file
- (dae0ce1a): Add search bar and page
- (c9d3d0d9): Add dockerfile and build jobs
- (8a10578b): Add remote url
- (8781158b): Add handler for unknown topic
- (86a64606): Add barcode to error message
- (a5aa4245): Add debug steps
- (bfda654a): Add debug step
- (66bb97ec): Add openfoodfactsapicontroller
populate object with data
- (071663b9): Add distinct api method to set ExpireAt
rename BestBefore to ExpireAt
add filter to api call
- (ddb607f4): Add gitlab ci
- (2573d006): Add mail notification
add mail template
change toAddress to []string
- (b44ee440): Add authenticaton middleware
add database controller
refactor model structure


### Changed
- (a9c162a6): Update config check and values
- (50f14ec8): Update namespace definition
- (b6c0ef6b): Update workflow
- (cd88a9f9): Update login step
- (6ff5fe16): Update ci tasks
- (15704b11): Update ci steps
- (27cebf76): Update ci
- (0a80e41e): Update CI
- (61b781b7): Update build file
- (d2f21233): Update path
- (8a600326): Update go version
- (ec68a0e7): Update actions
add template
- (ddd0934f): Update dockerfile
- (9a8e2202): Update context
- (64b57f9e): Update actions
- (be988674): Update CRUD methods
- (eec49436): Update name
- (1169f6e7): Update image
- (64950d2d): Update build target
- (7bc4cba5): Update stage
- (69b854b5): Update notifiedAt after notification is sent
- (4cf4730f): Update mail template
- (2bb971a2): Update var for privat push
- (ab94cd40): Update README
- (429e8001): Update README
relocate file
- (999fd45e): Update README
- (34cd3bf6): Update README
- (5c797f83): Update README
- (d759e275): Update README
- (e1edfdeb): Update README


### Fixed
- (deabf32c): Fix location of sln copy
- (69b9001f): Fix build
- (d597cab5): Fix typo
- (b5b29d05): Fix comments
fix names
update ci
- (85f333a2): Fix version
- (45b020bc): Fix naming
- (ed44960b): Fix push uri
- (01c6ebea): Fix docker build
- (efb03ea3): Fix ci
- (aadb11d8): Fix annotations
fix naming
- (22429237): Fix ci


### Removed
- (8ad219f7): Remove vscode
- (03d325be): Remove old folder
- (a6e10c65): Remove context
- (32a9a147): Remove step
- (d580e0ba): Remove exists rule
- (a649fa36): Remove html code
- (9fa5c8b8): Remove artifacts


<!-- generated by git-cliff -->

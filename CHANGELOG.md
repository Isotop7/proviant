# Changelog

All notable changes to `proviant` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Starting from v0.4.0, this file is auto-generated from [Conventional Commits](https://www.conventionalcommits.org/)
using [git-cliff](https://git-cliff.org/) when a release tag is pushed. Do not edit the auto-generated sections manually.

## [0.3.0] - 2025-12-26

### Added
- **Product Archive Management**
  - Archive products with soft deletion
  - Archive page with bulk selection and operations
  - Bulk restore and delete archived products
  - Interactive card selection with visual feedback
  
- **Enhanced Search**
  - Client-side product search
  - Enter key support for quick search
  - Improved search interface and navigation

- **Internationalization**
  - Support for all country codes
  - Country flag display functionality

- **UI Improvements**
  - Enhanced mobile menu with labels
  - Improved mobile view margins
  - Better button organization
  - Updated footer and empty state pages

### Fixed
- **Authentication**
  - Fixed login/signup errors now display properly
  - Resolved authentication issues

- **Search & Navigation**
  - Fixed search redirect problems
  - Removed duplicate HTML elements
  - Improved filtering functionality

- **Mobile Experience**
  - Fixed mobile view layout issues
  - Better responsive design on small screens

- **Data Management**
  - Fixed OpenFoodFacts API integration
  - Improved product deletion handling

### Removed
- Dedicated search page (integrated into main products view)

---

## [0.2.0] - 2024-11-30

### Added
- **Household Management**
  - Multi-household support with user assignments
  - Database migrations for household functionality
  - Scoped product management per household
  - User settings for household management

- **Live QR Code Scanning**
  - HTML5 QR code scanner integration
  - Real-time camera-based scanning
  - Improved scanning workflow and UI

- **Product Management Enhancements**
  - Product editing functionality
  - Product deletion with modal confirmation
  - Barcode-based product lookup
  - Product search and filtering

- **UI/UX Improvements**
  - Mobile-responsive QR code scanner
  - Enhanced product cards and navigation
  - Loading indicators and toasts
  - Fading alerts and improved error handling
  - Custom icons and favicon
  - Dark theme implementation

- **Developer Experience**
  - SQLite backend support
  - Docker Compose configurations (SQLite and MariaDB)
  - Comprehensive documentation
  - Makefile for build automation
  - Improved CI/CD pipeline

### Fixed
- Authentication and user registration flows
- Form validation and error handling
- Mobile responsive design issues
- Database query scoping and permissions
- Redirect handling and state management

### Changed
- Refactored project structure and moved backend files
- Updated Go modules and dependencies
- Standardized form layouts and styling
- Improved database controller architecture
- Enhanced logging and error handling

### Removed
- Legacy backend directory structure
- Unused authentication handlers

---

## [0.1.0] - Previous Version

[Unreleased]: https://codeberg.org/isotop7/proviant/compare/0.3.0...HEAD
[0.3.0]: https://codeberg.org/isotop7/proviant/compare/0.2.0...0.3.0
[0.2.0]: https://codeberg.org/isotop7/proviant/compare/v0.1...0.2.0
[0.1.0]: https://codeberg.org/isotop7/proviant/releases/tag/v0.1

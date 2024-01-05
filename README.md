![Expiro](./res/icons/logo_transparent.png){width=100%}

![License](https://img.shields.io/gitlab/license/Isotop7/expiro)
![Release](https://gitlab.com/Isotop7/expiro/-/badges/release.svg)
![CI @ main](https://gitlab.com/Isotop7/expiro/badges/main/pipeline.svg)
![CI @ develop](https://gitlab.com/Isotop7/expiro/badges/develop/pipeline.svg)
![Golang version](https://img.shields.io/badge/Go-1.21-green)


[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=Isotop7_expiro&metric=vulnerabilities)](https://sonarcloud.io/summary/new_code?id=Isotop7_expiro)
[![Bugs](https://sonarcloud.io/api/project_badges/measure?project=Isotop7_expiro&metric=bugs)](https://sonarcloud.io/summary/new_code?id=Isotop7_expiro)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Isotop7_expiro&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Isotop7_expiro)

⁉️ Ever wondered if this oat milk is still fresh or when you opened that hummus? In the past you may have thrown it away. Now there is **expiro**

📚 **expiro** is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food 🥗

## Features

- 📝 Track and log your products.
- 📊 Add information from OpenFoodFacts API.
- ⏰ Get reminders if products are due to expire.
- 📱 Mobile-friendly responsive design for on-the-go usage.

## Technologies and Tools

- **API:** Go, Gin, Gorm
- **Database:** MariaDB

## Installation and Usage

### Backend

#### Executable

1. Clone the repository: `git clone https://gitlab.com/Isotop7/expiro.git`
2. Navigate to the project directory: `cd expiro/src`
3. Build the application: `go build -o expiro`
4. Copy the config file `config.yaml.tmpl`, rename it to `config.yaml` and adjust it
5. Run the server: `./expiro`

#### Docker

`expiro` can be started with [docker compose](./docker-compose.yaml).

The app can be configured with environment variables:

```bash
# Server configuration
EXPIRO_SERVER_PORT=5050                                                     # Listening port of server
EXPIRO_SERVER_AUTHENTICATION_TOKENPASSWORD="secret key"                     # Secret used for JSON Web Tokens
EXPIRO_SERVER_AUTHENTICATION_TOKENLIFETIME=8                                # Lifetime of JSON Web Tokens

# Database configuration
EXPIRO_DATABASE_HOST="127.0.0.1"                                            # Database server IP or hostname
EXPIRO_DATABASE_PORT=3306                                                   # Database server port
EXPIRO_DATABASE_NAME="expiro"                                               # Database name
EXPIRO_DATABASE_USER="expiro"                                               # Database user
EXPIRO_DATABASE_PASSWORD="password"                                         # Database user password

# Logging configuration
EXPIRO_LOGGING_ENABLED=true                                                 # Enable file logging
EXPIRO_LOGGING_FILE="expiro.log"                                            # Path to log file

# Notification configuration
EXPIRO_NOTIFICATION_ENABLED=true                                            # Enable notifications
EXPIRO_NOTIFICATION_INTERVAL=12                                             # Interval in hours when notifications should be send
EXPIRO_NOTIFICATION_FROMADDRESS="sender@local.net"                          # Sender address for notifications
EXPIRO_NOTIFICATION_SMTP_HOST="127.0.0.1"                                   # Host or IP address of SMTP server
EXPIRO_NOTIFICATION_SMTP_PORT=25                                            # Port of SMTP server
EXPIRO_NOTIFICATION_SMTP_SSL=false                                          # Enables/disables SSL
EXPIRO_NOTIFICATION_SMTP_USER="user"                                        # Username used for sending notifications via SMTP server
EXPIRO_NOTIFICATION_SMTP_PASSWORD="password"                                # Password used for sending notifications via SMTP server

# OpenFoodFacts configuration
EXPIRO_OPENFOODFACTS_URL="https://world.openfoodfacts.org/api/v2/product"   # Address of API backend of OpenFoodFacts
EXPIRO_OPENFOODFACTS_TIMEOUT=5                                              # Timeout of API requests to OpenFoodFacts API
```

Additionally `Gin` supports a debug mode, which also can be set with a environment variable:

```bash
GIN_MODE=debug
```

## What's missing?

- Authentication: Add authentication on the Web UI so only you can edit and delete your digital fridge
- Administrative Functions: Create and Update users
- Personalize fridge: Assign products to users or user groups, send notifications to assigned entity
- Documentation: Provide screenshots and basic help guide
- Automated testing: Provide automated testing for better code qualitxy

## Documentation

Documentation is generated with `gomarkdoc`

- [Package documentation](./src/docs/README.md)
- [Swagger definition](./src/docs/swagger.yaml) 

## Contributing

Contributions are welcome! Fork the repository, make your changes, and submit a pull request.

## License

This project is licensed under the [MIT License](LICENSE).

## Author

- GitLab: [@Isotop7](https://gitlab.com/Isotop7)
- LinkedIn: [Hendrik Röder](https://www.linkedin.com/in/hendrik-r%C3%B6der-9b8483198/)

---

⭐️ If you find this project helpful, give it a star and share it with others! ⭐️

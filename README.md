{::options parse_block_html="true" /}

<h1 align="center">expiro</h1>

<p align="center">
    <img src="https://img.shields.io/gitlab/license/Isotop7/expiro" alt="License" />
    <img src="https://gitlab.com/Isotop7/expiro/-/badges/release.svg" alt="Release" />
    <img src="https://gitlab.com/Isotop7/expiro/badges/mnain/pipeline.svg" alt="CI @ main" />
    <img src="https://gitlab.com/Isotop7/expiro/badges/develop/pipeline.svg" alt="CI @ develop" />
    <img src="https://img.shields.io/badge/Go-1.21-green" alt="Golang version>
</p>

<p align="center">
    ⁉️ Ever wondered if this oat milk is still fresh or when you opened that hummus? In the past you may have thrown it away. Now there is <b>expiro</b>
</p>
<p align="center">
    📚 <b>expiro</b> is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food 🥗
</p>

## 🚀 Features

- 📝 Track and log your products.
- 📊 Add information from OpenFoodFacts API.
- ⏰ Get reminders if products are due to expire.
- 📱 Mobile-friendly responsive design for on-the-go usage.

## 🛠️ Technologies and Tools

- **Frontend:** ~~Razor~~, HTML, CSS
- **Backend:** Go, Gin, Gorm
- **Database:** MariaDB or SQLite (fallback)

## 📦 Installation and Usage

### 🔙 Backend

1. Clone the repository: `git clone https://gitlab.com/Isotop7/expiro.git`
2. Navigate to the project directory: `cd expiro/backend`
3. Build the application: `go build -o expiro-backend`
4. Copy the config file `config.yaml.tmpl`, rename it to `config.yaml` and adjust it
5. Run the server: `./expiro-server`
6. ~~Start frontend server~~

### 🖼️ Frontend

### 💻 Client

*TODO*

## 😥 What's missing?

### 🔙 Backend

- Authentication: Add authentication on the Web UI so only you can edit and delete your digital fridge
- Administrative Functions: Create and Update backend users
- Personalize fridge: Assign products to users or user groups, send notifications to assigned entity
- Documentation: Provide screenshots and basic help guide
- Automated testing: Provide automated testing for better code qualitxy

### 💻 Client

- `expiro-esp`: Reference implementation of a ESP microcontroller and a connected barcode scanner to check in products

## 🤝 Contributing

Contributions are welcome! Fork the repository, make your changes, and submit a pull request.

## 📄 License

This project is licensed under the [MIT License](LICENSE).

## 👤 Author

- GitHub: [@Isotop7](https://github.com/Isotop7)
- LinkedIn: [Hendrik Röder](https://www.linkedin.com/in/hendrik-r%C3%B6der-9b8483198/)

---

⭐️ If you find this project helpful, give it a star and share it with others! ⭐️

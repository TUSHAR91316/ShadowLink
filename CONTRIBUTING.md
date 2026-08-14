# 🤝 Contributing to ShadowLink

Thank you for contributing to ShadowLink! As a fully decentralized, community-driven privacy network, ShadowLink relies on contributors like you to maintain code quality, audit security, and build new capabilities.

---

## 📋 Code of Conduct

All contributors and maintainers are expected to adhere to our [Code of Conduct](CODE_OF_CONDUCT.md). Please read it before participating.

---

## 🛠️ Development Environment

### Prerequisites
- **Go**: Version 1.22 or higher ([Download](https://go.dev/dl/))
- **Flutter SDK**: Version 3.10 or higher ([Download](https://flutter.dev/docs/get-started/install))
- **Git**: For source control

---

## 🧪 Testing Guidelines

Before opening a pull request, ensure that all tests pass cleanly without errors, race conditions, or lint warnings.

### 1. Go Core Engine
```bash
# Run all unit tests with data race detector enabled
go test -race -v ./...

# Run static analysis
go vet ./...

# Ensure code is formatted to Go standards
go fmt ./...
```

### 2. Flutter GUI
```bash
cd shadowlink_gui

# Run static analysis and linter
flutter analyze

# Run Flutter unit and widget tests
flutter test
```

---

## 🌿 Contribution Workflow

1. **Fork the Repository**: Create your own fork of `TUSHAR91316/ShadowLink`.
2. **Create a Topic Branch**:
   ```bash
   git checkout -b feature/add-transport-obfuscation
   ```
3. **Write Clean, Documented Code**:
   - Adhere to Effective Go standards and Flutter recommended practices.
   - Never introduce hardcoded values; define protocol constants in `internal/config/config.go` or `lib/config/app_config.dart`.
   - Ensure all cryptographic operations utilize domain-separated HKDF or standard AEAD routines.
4. **Commit with Descriptive Messages**:
   ```bash
   git commit -m "feat(network): add multi-frame fragmentation support"
   ```
5. **Rebase on Main**:
   ```bash
   git checkout main
   git pull upstream main
   git checkout feature/add-transport-obfuscation
   git rebase main
   ```
6. **Submit a Pull Request**: Provide a clear explanation of what your PR accomplishes, including verification steps and test outputs.

---

## 🔒 Security Vulnerability Reporting

If you discover a potential security flaw or cryptographic vulnerability, please **do not open a public issue**. Instead, submit a responsible disclosure report directly to the core maintainers.

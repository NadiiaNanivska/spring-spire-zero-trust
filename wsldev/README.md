# WSLDev CLI Tool

## Overview
WSLDev is a CLI tool designed to simplify development workflows. It provides utilities for managing applications, interacting with Docker, and working with Kubernetes clusters.

## Prerequisites
- Go 1.17 or higher installed on your system.
- Docker installed and running.
- Kubernetes cluster configured (if using Kubernetes-related features).

## Installation
1. Clone the repository:
   ```bash
   git clone https://github.com/your-repo/wsldev.git
   ```
2. Navigate to the project directory:
   ```bash
   cd wsldev
   ```
3. Build the CLI tool:
   ```bash
   go build -o wsldev ./cmd/wsldev
   ```

## Usage
After building the tool, you can run it using the following command:
```bash
wsldev [command] [flags]
```

### Available Commands

#### 1. Application Deployment
Deploy applications using the `app_deploy` command:
```bash
wsldev app_deploy --config <config-file>
```
- `--config`: Path to the configuration file for deployment.

#### 2. Docker Daemon Interaction
Interact with the Docker daemon using the `daemon` command:
```bash
wsldev daemon start       # Запустити Docker daemon у WSL
wsldev daemon status      # Перевірити, чи Docker запущено```
```

#### 3. Kubernetes Cluster Management
Manage Kubernetes clusters using the `kubernetes` command:
```bash
wsldev cluster create --name kind   # Створити Kind кластер
wsldev cluster delete --name kind   # Видалити кластер
wsldev cluster reset --name kind    # Повний reset (delete + create)
wsldev cluster info --name kind     # Показати інформацію про кластер 
wsldev up --name kind   # Підняти Docker та Kubernetes кластер одним викликом
```

#### 4. SPIRE Infrastructure Management
Manage SPIRE Infrastructure using the `spire` command:
```bash
wsldev spire deploy             # Розгорнути SPIRE
wsldev spire entry create       # Створити новий entry у SPIRE
wsldev spire entry show         # Показати список entry
wsldev spire svid rotate        # Форсована ротація SVID (планується)
```

#### 5. Backend Apps Management
Manage Spring apps using the `app` command:
```bash
wsldev app deploy [app names]   # Розгорнути сервіс
wsldev app logs                 # Показати логи сервісу
wsldev app port-forward         # Проброс портів на локальну машину
```

## Configuration
Configuration files are used to define application deployment settings. Ensure your configuration files are in YAML or JSON format and include all required fields.

## Logging
Logs are generated for all operations and can be found in the `logs/` directory. Ensure the directory exists before running the tool.

## Contributing
1. Fork the repository.
2. Create a new branch for your feature or bug fix:
   ```bash
   git checkout -b feature-name
   ```
3. Commit your changes:
   ```bash
   git commit -m "Description of changes"
   ```
4. Push to your branch:
   ```bash
   git push origin feature-name
   ```
5. Create a pull request.

## License
This project is licensed under the MIT License. See the LICENSE file for details.

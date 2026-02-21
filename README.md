# OIS — Orchestrated Infrastructure Scaffolder

Build production-ready AWS stacks in seconds. An interactive Go-driven CLI that automates Terraform scaffolding, environment isolation, and service interfaces.

**Website:** [oisbase.dev](https://oisbase.dev) · **Documentation:** [oisbase.dev/docs](https://oisbase.dev/docs)

## Features

- **Zero to Infrastructure in Minutes** — Interactive prompts guide you through setup
- **Multi-Environment by Default** — Separate dev/staging/prod from the start
- **Service Isolation** — Each service type manages its own state
- **Incremental Development** — Add services as you need them
- **Production-Ready Templates** — Battle-tested Terraform modules
- **Best Practices Built-In** — Proper state management, tagging, and structure
- **Cost Estimation** — Estimate infrastructure costs via OpenInfraQuote (`ois estimate`)

## Installation

### Prerequisites

- Go 1.21+ (for from-source installation)
- Terraform (for deploying generated configurations)

### Homebrew (macOS/Linux)

```bash
brew tap NipulM/ois
brew install ois
```

### From Source

```bash
go install github.com/NipulM/oisbase@latest
```

> **Note:** The binary is named `oisbase`. Create an alias if you prefer: `alias ois=oisbase`

## Quick Start

```bash
# Create a new project
mkdir my-infrastructure
cd my-infrastructure
ois init

# Add a Lambda function
ois add lambda

# Add more services
ois add dynamodb
```

To estimate costs after generating your infrastructure:

```bash
terraform plan -out=tf.plan
ois estimate
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `ois init` | Initialize a new Terraform project with interactive prompts |
| `ois add [service]` | Add a service instance (e.g., `lambda`, `dynamodb`) |
| `ois add [service] --template` | Copy the Terraform module when the service wasn't selected during init |
| `ois estimate` | Estimate costs using OpenInfraQuote (requires `terraform plan -out=tf.plan`) |

## Supported Services

| Service | Status | Description |
|---------|--------|-------------|
| Lambda | Available | Serverless functions |
| DynamoDB | Available | NoSQL database |
| RDS | Coming soon | Relational database |
| VPC | Coming soon | Virtual private cloud |
| ECS | Coming soon | Container orchestration |
| S3 | Coming soon | Object storage |

## Generated Project Structure

After running `ois init` and adding services, your project will look like:

```
project-name/
├── environments/
│   ├── pre-production/
│   │   ├── dev/
│   │   │   ├── lambda/
│   │   │   └── dynamodb/
│   │   └── stg/
│   └── production/
│       └── prod/
├── modules/
│   ├── lambda/
│   └── dynamodb/
├── main.tf
├── backend.tf
└── .oisbase.json
```

## Prerequisites for Deployment

Before deploying with Terraform, you'll need to set up:

1. **S3 bucket** for Terraform state
2. **DynamoDB table** for state locking

See the [documentation](https://oisbase.dev/docs) for step-by-step guides and service-specific prerequisites.

## License

MIT

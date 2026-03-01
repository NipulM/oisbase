# OIS — Infrastructure Scaffolding CLI for IaC Platforms

Build production-ready AWS infrastructure in seconds.

ois is an **opinionated AWS Terraform project generator CLI** that scaffolds
multi-environment infrastructure, service modules, and state isolation using best practices.

**Website:** [oisbase.dev](https://oisbase.dev) · **Documentation:** [oisbase.dev/docs](https://oisbase.dev/docs)

## Features

- **Zero to Infrastructure in Minutes** — Interactive prompts guide you through setup
- **Multi-Environment by Default** — Separate dev/staging/prod from the start
- **Service Isolation** — Each service type manages its own state
- **Cross-Service Connections** — Auto-generated IAM policies, SSM lookups, and event triggers when you connect services
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

# Add a DynamoDB table and connect it to your Lambda
ois add dynamodb

# Add an SQS queue with Lambda trigger
ois add sqs

# Add an API Gateway with routes to your Lambda
ois add api-gateway
```

When you connect services during `ois add`, the CLI automatically generates the IAM policies, SSM parameter lookups, and event source mappings needed for them to work together.

To estimate costs after generating your infrastructure:

```bash
terraform plan -out=tf.plan
ois estimate
```

## CLI Reference

| Command                        | Description                                                                  |
| ------------------------------ | ---------------------------------------------------------------------------- |
| `ois init`                     | Initialize a new Terraform project with interactive prompts                  |
| `ois add [service]`            | Add a service instance (e.g., `lambda`, `dynamodb`, `api-gateway`, `sqs`)    |
| `ois add [service] --template` | Copy the Terraform module when the service wasn't selected during init       |
| `ois estimate`                 | Estimate costs using OpenInfraQuote (requires `terraform plan -out=tf.plan`) |

## Supported Services

| Service     | Status      | Description                      |
| ----------- | ----------- | -------------------------------- |
| Lambda      | Available   | Serverless functions             |
| DynamoDB    | Available   | NoSQL database                   |
| API Gateway | Available   | HTTP API with route-based config |
| SQS         | Available   | Message queuing                  |
| S3          | Coming Soon | Object storage                   |

## Generated Project Structure

After running `ois init` and adding services, your project will look like:

```
acme-payments/
├── environments/
│   ├── pre-production/
│   │   ├── dev/
│   │   │   ├── lambda/
│   │   │   │   ├── payment-processor/
│   │   │   │   │   ├── main.tf
│   │   │   │   │   ├── variables.tf
│   │   │   │   │   ├── iam.tf
│   │   │   │   │   ├── data.tf
│   │   │   │   │   └── triggers.tf
│   │   │   │   └── notification-service/
│   │   │   ├── dynamodb/
│   │   │   │   └── transactions/
│   │   │   ├── sqs/
│   │   │   │   └── payment-events/
│   │   │   └── api-gateway/
│   │   │       └── acme-payments-api/
│   │   │           ├── main.tf
│   │   │           ├── variables.tf
│   │   │           ├── outputs.tf
│   │   │           ├── iam.tf
│   │   │           ├── data.tf
│   │   │           └── api.yaml
│   │   └── stg/
│   └── production/
│       └── prod/
├── modules/
│   ├── lambda/
│   ├── dynamodb/
│   └── sqs/
└── .oisbase.json
```

## Prerequisites for Deployment

Before deploying with Terraform, you'll need to set up:

1. **S3 bucket** for Terraform state
2. **DynamoDB table** for state locking

See the [documentation](https://oisbase.dev/docs) for step-by-step guides and service-specific prerequisites.

## When Should I Use ois?

Use ois if you:

- Are starting a new AWS Terraform project
- Want a consistent, production-ready structure
- Need multi-environment isolation from day one
- Prefer opinionated defaults over custom boilerplate

ois is especially useful for teams standardizing AWS infrastructure.

## License

MIT

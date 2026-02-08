# OIS - Orchestrated Infrastructure Scaffolder

An interactive CLI tool that scaffolds production-ready Terraform configurations for AWS infrastructure.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap NipulM/ois
brew install ois
```

<!-- ### From Source

```bash
go install github.com/NipulM/ois@latest
``` -->

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
ois add rds
```

## Features

- 🚀 **Zero to Infrastructure in Minutes** - Interactive prompts guide you through setup
- 🏗️ **Multi-Environment by Default** - Separate dev/staging/prod from the start
- 📦 **Service Isolation** - Each service type manages its own state
- 🔧 **Incremental Development** - Add services as you need them
- 📋 **Production-Ready Templates** - Battle-tested Terraform modules
- 🎯 **Best Practices Built-In** - Proper state management, tagging, and structure

## Supported Services

- Lambda
- DynamoDB (coming soon)
- RDS (coming soon)
- VPC (coming soon)
- ECS (coming soon)
- S3 (coming soon)

## License

MIT

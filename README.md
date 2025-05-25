# go-musthave-metrics-tpl

Repository template for the "Metrics Collection and Alerting Server" track.

## How to Run Tests Locally

### Run All Tests
```bash
go test -v ./...
```

### Run Tests for Specific Components
```bash
# Server tests only
go test -v ./cmd/server

# Agent tests only
go test -v ./cmd/agent
```

### Run Specific Tests
```bash
# Run only the UpdateHandler test
go test -v ./cmd/server -run TestUpdateHandler

# Run only the PollMetrics test
go test -v ./cmd/agent -run TestPollMetrics
```

### Static Code Analysis
```bash
# Check with go vet
go vet ./...

# Check import formatting
go install golang.org/x/tools/cmd/goimports@v0.20.0
goimports -l .
```

## Getting Started

1. Clone the repository to any suitable directory on your computer.
2. In the repository root, run the command `go mod init <name>` (where `<name>` is your GitHub repository address without the `https://` prefix) to create a module.

## Template Updates

To be able to receive updates for autotests and other parts of the template, run the command:

```
git remote add -m main template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

To update the autotest code, run the command:

```
git fetch template && git checkout template/main .github
```

Then add the received changes to your repository.

## Running Autotests

For successful autotest execution, name branches `iter<number>`, where `<number>` is the increment sequence number. For example, in a branch named `iter4`, autotests for increments one through four will run.

When merging an increment branch into the main `main` branch, all autotests will run.

For more details about local and automatic execution, read the [autotests README](https://github.com/Yandex-Practicum/go-autotests).

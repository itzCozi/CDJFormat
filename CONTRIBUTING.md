# Contributing to CDJFormat

Thank you for your interest in contributing to CDJFormat! This document provides guidelines for contributing to the project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/CDJFormat.git`
3. Create a new branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Test your changes thoroughly
6. Commit your changes: `git commit -m "Add your feature"`
7. Push to your fork: `git push origin feature/your-feature-name`
8. Create a Pull Request

## Development Setup

### Requirements

- Go 1.20 or later
- Git

### Building

```bash
go build -o cdjformat
```

### Testing

Test the CLI commands:

```bash
# List drives
./cdjformat list

# Format help
./cdjformat format --help
```

## Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Run `go vet` to check for common mistakes
- Write clear, descriptive commit messages

## Commit Messages

- Use present tense ("Add feature" not "Added feature")
- Use imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests when applicable

## Pull Request Process

1. Update the README.md with details of changes if applicable
2. Ensure your code builds and runs correctly
3. Test on your platform (Linux/macOS/Windows)
4. Describe your changes clearly in the PR description
5. Link any related issues

## Feature Requests

We welcome feature requests! Please:

1. Check existing issues first to avoid duplicates
2. Clearly describe the feature and its use case
3. Explain why this feature would be useful to other users

## Bug Reports

When reporting bugs, please include:

1. Your operating system and version
2. Go version (`go version`)
3. Steps to reproduce the issue
4. Expected behavior
5. Actual behavior
6. Any error messages or logs

## Questions?

Feel free to open an issue for any questions about contributing.

## License

By contributing to CDJFormat, you agree that your contributions will be licensed under the same license as the project.

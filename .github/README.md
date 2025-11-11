# GitHub Configuration

This directory contains GitHub-specific configuration files for the Bananas project.

## 📁 Directory Structure

```
.github/
├── workflows/              # GitHub Actions workflows
│   ├── go-ci.yml          # Go tests, linting, security
│   ├── python-sdk-ci.yml  # Python SDK tests & linting
│   ├── typescript-sdk-ci.yml  # TypeScript SDK tests & linting
│   ├── integration-tests.yml  # End-to-end integration tests
│   ├── docker.yml         # Docker build & security scanning
│   ├── docs.yml           # Documentation checks
│   ├── codeql.yml         # Security scanning with CodeQL
│   ├── labeler.yml        # Auto-label PRs
│   ├── benchmarks.yml     # Performance benchmarks
│   ├── stale.yml          # Stale issue management
│   └── greetings.yml      # Welcome first-time contributors
├── ISSUE_TEMPLATE/        # Issue templates
│   ├── bug_report.md      # Bug report template
│   ├── feature_request.md # Feature request template
│   └── config.yml         # Issue template config
├── dependabot.yml         # Dependency updates config
├── labeler.yml            # PR auto-labeling rules
└── pull_request_template.md  # PR template
```

## 🔄 Workflows

### Core CI/CD Workflows

#### Go CI (`go-ci.yml`)
**Triggers:** Every PR and push to main
- **Test Job:** Runs tests with Go 1.21 and 1.22, coverage checking (80% threshold)
- **Lint Job:** golangci-lint, go vet, format check, go.mod tidy check
- **Security Job:** govulncheck, Trivy vulnerability scanning

#### Python SDK CI (`python-sdk-ci.yml`)
**Triggers:** PRs affecting `sdk/python/**`
- **Test Job:** Matrix testing (Python 3.8-3.12, Ubuntu/macOS/Windows), 90% coverage threshold
- **Lint Job:** black, isort, pylint (8.0 score), mypy type checking
- **Build Job:** Package building and verification
- **Security Job:** bandit, safety checks

#### TypeScript SDK CI (`typescript-sdk-ci.yml`)
**Triggers:** PRs affecting `sdk/typescript/**`
- **Test Job:** Matrix testing (Node 16/18/20, Ubuntu/macOS/Windows)
- **Lint Job:** TypeScript type checking, ESLint, Prettier
- **Build Job:** Package building and verification
- **Security Job:** npm audit

#### Integration Tests (`integration-tests.yml`)
**Triggers:** Every PR, push to main, nightly schedule
- End-to-end testing with real Redis
- Cross-language SDK testing
- Docker Compose integration tests

### Supporting Workflows

#### CodeQL (`codeql.yml`)
**Triggers:** PR, push to main, weekly schedule
- Security scanning for Go, JavaScript, and Python
- Automated vulnerability detection
- Results uploaded to GitHub Security tab

#### Docker (`docker.yml`)
**Triggers:** PR, push to main, releases
- Build scheduler and worker images
- Security scanning with Trivy
- Image testing

#### Documentation (`docs.yml`)
**Triggers:** PRs affecting docs or markdown files
- Markdown linting
- Link checking
- Spell checking
- Code example validation

#### Benchmarks (`benchmarks.yml`)
**Triggers:** PR, push to main, manual
- Go benchmark tests
- Performance comparison with baseline
- Load testing

#### PR Labeler (`labeler.yml`)
**Triggers:** PR open/sync
- Auto-labels based on changed files
- Size labeling (XS/S/M/L/XL)

#### Stale (`stale.yml`)
**Triggers:** Daily schedule
- Marks stale issues (60 days inactive)
- Marks stale PRs (45 days inactive)
- Auto-closes after warning period

#### Greetings (`greetings.yml`)
**Triggers:** First-time issue or PR
- Welcomes new contributors
- Provides guidance and resources

## 🤖 Dependabot

Automated dependency updates for:
- **Go modules** (weekly, Mondays)
- **GitHub Actions** (weekly, Mondays)
- **Python dependencies** (weekly, Mondays)
- **npm packages** (weekly, Mondays)
- **Docker base images** (weekly, Mondays)

## 🏷️ PR Auto-Labeling

PRs are automatically labeled based on changed files:
- `go` - Go code changes
- `python-sdk` - Python SDK changes
- `typescript-sdk` - TypeScript SDK changes
- `documentation` - Documentation changes
- `ci/cd` - CI/CD changes
- `tests` - Test changes
- `dependencies` - Dependency updates
- `docker` - Docker-related changes
- `examples` - Example code changes
- `security` - Security-related changes

Size labels are also automatically applied:
- `size/XS` - 1-10 lines
- `size/S` - 11-100 lines
- `size/M` - 101-500 lines
- `size/L` - 501-1000 lines
- `size/XL` - 1000+ lines

## 📝 Templates

### Pull Request Template
Ensures PRs include:
- Description and type of change
- Related issues
- Testing checklist
- Performance impact
- Breaking changes
- Screenshots (if applicable)

### Issue Templates

**Bug Report:**
- Description
- Reproduction steps
- Environment details
- Code samples
- Error logs

**Feature Request:**
- Problem statement
- Proposed solution
- Use case
- Example usage
- Implementation ideas

## 🔒 Security

Security scanning is performed at multiple levels:
- **CodeQL:** Static analysis for Go, JavaScript, Python
- **Trivy:** Container and dependency vulnerability scanning
- **govulncheck:** Go vulnerability checking
- **npm audit:** npm dependency checking
- **bandit/safety:** Python security checking

Results are uploaded to GitHub Security tab for review.

## ✅ Required Status Checks

For a PR to be merged, the following checks must pass:
- All test suites (Go, Python SDK, TypeScript SDK)
- All linters
- Security scans
- Docker builds
- Coverage thresholds

## 🎯 Coverage Thresholds

- **Go:** 80% minimum
- **Python SDK:** 90% minimum
- **TypeScript SDK:** 85% minimum

## 🚀 Continuous Deployment

Release workflow will be triggered on version tags:
- Build multi-platform binaries
- Publish Python package to PyPI
- Publish TypeScript package to npm
- Build and push Docker images

## 📊 Artifacts

Workflows upload artifacts for:
- Coverage reports (Codecov)
- Benchmark results
- Build packages (Python wheel, npm tarball)
- Security scan results

## 🔧 Configuration Files

- `.golangci.yml` - golangci-lint configuration
- `.markdownlint.json` - Markdown linting rules
- `.markdown-link-check.json` - Link checker config
- `.spellcheck.yml` - Spell check configuration
- `.wordlist.txt` - Custom dictionary for technical terms

## 🤝 Contributing

When adding new workflows:
1. Test locally if possible
2. Start with `workflow_dispatch` trigger for testing
3. Add appropriate caching to speed up runs
4. Document the workflow purpose and triggers
5. Add to this README

## 📚 Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [GitHub Actions Marketplace](https://github.com/marketplace?type=actions)
- [Dependabot Documentation](https://docs.github.com/en/code-security/dependabot)
- [CodeQL Documentation](https://codeql.github.com/docs/)

# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Instead, report privately via GitHub's
[private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
("Report a vulnerability" under the repository's **Security** tab), or email the
maintainer.

We will acknowledge receipt within a few business days and keep you updated on
remediation.

## Scope

Paper Trail handles API keys (OpenAI, Brave) and connects to external sources.
Relevant concerns include:

- Secret/credential leakage (keys must live only in gitignored `.env`).
- SSRF / unsafe URL handling in the fetch pipeline.
- SQL injection in store queries.
- Template injection / XSS in the review dashboard (`internal/api`, `web/`).

## Responsible use

Paper Trail is a tool for analysing **public** material. Operators are
responsible for respecting each source's `robots.txt` and Terms of Service, for
not bypassing access controls, and for not redistributing copyrighted content.
See [`docs/03-compliance.md`](./docs/03-compliance.md).

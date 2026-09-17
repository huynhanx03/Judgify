# Security Policy

Judgify is an online judge platform that handles user accounts, code submissions, contest data, and sandboxed code execution. Security reports are taken seriously, especially issues that affect authentication, authorization, submitted code execution, data exposure, or platform integrity.

## Supported Versions

Judgify is currently under active development. Security fixes are handled on the latest `main` branch unless a maintained release branch is announced in the future.

## Reporting a Vulnerability

Please do not open a public GitHub issue for security vulnerabilities.

Report security concerns through GitHub Security Advisories:

[Report a vulnerability](https://github.com/huynhanx03/Judgify/security/advisories/new)

Include as much relevant detail as possible:

- Affected area, endpoint, package, or feature
- Steps to reproduce the issue
- Expected and actual behavior
- Potential impact
- Any proof of concept, logs, screenshots, or request samples
- Suggested fix or mitigation, if you have one

## What to Report

Examples of valid security reports include:

- Authentication or session bypass
- Broken access control or privilege escalation
- Sensitive data exposure
- Code execution sandbox escape
- Unsafe execution of submitted code
- SQL injection, command injection, XSS, CSRF, or SSRF
- Dependency vulnerabilities that affect the running application
- Secrets, tokens, credentials, or private keys committed to the repository

## Responsible Disclosure

Please give maintainers reasonable time to investigate and patch the issue before public disclosure. When testing, avoid accessing, modifying, deleting, or exfiltrating data that does not belong to you.

If a report is confirmed, the fix will be prioritized based on severity, exploitability, and affected surface area.

## Out of Scope

The following are usually not treated as security vulnerabilities:

- General bugs without security impact
- Missing best-practice headers without a practical exploit
- Reports from automated scanners without reproduction steps
- Denial-of-service claims without clear impact or realistic constraints
- Vulnerabilities that require full local machine access

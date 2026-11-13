# Security Policy

We take security seriously at Entire. We appreciate your efforts to responsibly disclose vulnerabilities and will make every effort to acknowledge your contributions.

## Reporting a Vulnerability

**Please do not report security vulnerabilities publicly.**

Instead, please send security-related reports to **[security@entire.com](mailto:security@entire.com)**, the contact published in [Entire's security.txt](https://entire.com/.well-known/security.txt).

### What to Include

When reporting a vulnerability, please include:

1. **Description** - A clear description of the vulnerability
2. **Impact** - What an attacker could achieve by exploiting this issue
3. **Steps to reproduce** - Detailed steps or code to reproduce the vulnerability
4. **Affected versions** - Which versions or commits of the SDK are affected (if known)
5. **Suggested fix** - If you have ideas on how to fix it (optional)

### What to Expect

- **Acknowledgment** - We will acknowledge receipt of your report within 48 hours
- **Updates** - We will keep you informed of our progress as we investigate
- **Resolution** - We aim to resolve critical vulnerabilities within 90 days
- **Credit** - We will credit you in the release notes if you wish

## Confidentiality

**All reports will be kept confidential.** We will not share your information with third parties without your consent, except as required by law.

## Supported Versions

Security fixes are made on `main` and in the latest release. We recommend always using the latest version of the SDK.

## Scope

This security policy applies to:

- The Entire Go SDK (`go.entire.com/entire-sdk-go`), including the `plugin` package
- Official Entire repositories
- Entire-related services at entire.com

Reports about the SDK itself include, for example:

- Credentials sent to a host other than the configured API, or through a redirect
- Path or parameter handling that lets input change the request's route
- Unbounded reads or other resource exhaustion caused by server responses
- Leaks of tokens or response data through errors or logs
- Webhook signatures, receipts, or tokens that the `plugin` package accepts when it should reject them

### Out of Scope

The following are generally not considered security vulnerabilities:

- Issues in third-party dependencies (please report these upstream)
- Social engineering attacks
- Denial of service attacks against Entire services
- Issues requiring physical access to a user's device

---

Thank you for helping keep Entire and our community safe!

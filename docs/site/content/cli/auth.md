---
title: "auth"
weight: 40
---

# auth

Manage authentication with the registry. Uses the OIDC device authorization flow (RFC 8628).

## Synopsis

```
skillsctl auth login
skillsctl auth status
skillsctl auth logout
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `login` | Authenticate and cache credentials |
| `status` | Check current authentication state |
| `logout` | Delete cached credentials |

## login

Fetches OIDC configuration from the server (`GET /auth/config`), then starts a device authorization flow. You are given a URL and a user code to enter in a browser. Once you authorize in the browser, the CLI receives a token and caches it at `~/.config/skillsctl/credentials.json`.

```bash
skillsctl auth login
```

```
Open this URL in your browser:
  https://auth.example.com/device

Enter code: ABCD-1234

Waiting for authorization...
Logged in as alice@example.com
```

## status

Prints the current authentication state and exits with code 0 if the token is valid, or code 1 if no credentials exist or the token has expired.

```bash
skillsctl auth status
```

```
Logged in as alice@example.com
Token expires: 2026-04-18 10:00:00 UTC
```

If not logged in:

```bash
skillsctl auth status
echo $?
```

```
Not logged in.
1
```

## logout

Deletes the credentials file. The next command that requires authentication will prompt you to log in again.

```bash
skillsctl auth logout
```

```
Logged out.
```

## Credentials file

Tokens are cached at `~/.config/skillsctl/credentials.json`. Use `--credentials-path PATH` to specify a different location.

## Common errors

**Error: server does not have OIDC configured**
The registry is running in dev mode without authentication. No login is required or possible.

**Error: failed to fetch auth config from server: ...**
The CLI could not reach the server. Check `skillsctl config get api_url`.

**Authorization timed out.**
You did not complete the browser authorization within the allowed window. Run `skillsctl auth login` again.

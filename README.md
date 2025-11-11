# Caddy JWT Signer

A Caddy v2 module that generates and signs a JSON Web Token (JWT) and makes it available to other directives via a
replacer.

This is useful for scenarios where you need to dynamically create a token for a request, such as for authentication
with a backend service or for passing state in a redirect.

## Installation

Build Caddy with this module:

```bash
xcaddy build --with github.com/CthulhuDen/caddy-jwt-signer
```

## Caddyfile Syntax

```caddyfile
jwt_signer <duration> <secret> {
    <key> <value>
    <key> {
        <nested_key> <nested_value>
    }
    ...
}
```

*   **`<duration>`**: The duration for which the token will be valid (e.g., `15m`, `1h`). This can be a
    placeholder.
*   **`<secret>`**: The secret key to sign the token with. This can be a placeholder.
*   The block contains the claims to include in the JWT payload. The `iat` (issued at) and `exp` (expiration) claims
    are automatically added. String values can be replacer placeholders. Nested claims are supported.

## Replacer

The signed token is made available via the `{http.jwt_signer.digest_str}` placeholder.

## Directive Order

The `jwt_signer` directive is ordered to run before the `redir` directive by default. This allows you to use the
generated token in a redirect URL.

For more complex scenarios, like using it with `forward_auth`, you might need to control the execution order
explicitly. A common use case is to run `forward_auth` first to authenticate the user and get user information, then
use `jwt_signer` to create a token with that information. This can be achieved by using a `route` block and ordering
the directives accordingly.

## Examples

### Basic Usage

This configuration will generate a token and add it to the `X-Auth-Token` header for requests to the backend.

```caddyfile
example.com {
    jwt_signer 1h {$JWT_SECRET} {
        sub user@example.com
        name "John Doe"
        admin true
    }
    reverse_proxy backend:8080 {
        header_up X-Auth-Token {http.jwt_signer.digest_str}
    }
}
```

### Redirect with Token

This configuration signs a token with only `iat` and `exp` claims, and passes it as a query parameter in a
redirect.

```caddyfile
example.com {
    jwt_signer 15m {$JWT_SECRET}
    redir https://auth.example.com/login?token={http.jwt_signer.digest_str}
}
```
### Usage with `forward_auth`

This example shows how to use `forward_auth` to authenticate a user, and then use `jwt_signer` to create a token
containing user information obtained from `forward_auth`. The token is then used in a redirect. The `route` block is
used to enforce the directive order.

```caddyfile
example.com {
    route {
        forward_auth authelia:9091 {
            uri /api/verify?rd=https://login.example.com/
            copy_headers Remote-User Remote-Groups Remote-Name Remote-Email
        }

        jwt_signer 10m {$JWT_SECRET} {
            sub {http.request.header.Remote-User}
            groups {http.request.header.Remote-Groups}
            name {http.request.header.Remote-Name}
            email {http.request.header.Remote-Email}
        }

        redir https://service.example.com/dashboard?token={http.jwt_signer.digest_str}
    }
}
```

### Nested Claims and Placeholders

You can also define nested claims. Placeholders can be used in values.

```caddyfile
example.com {
    jwt_signer 1h {$JWT_SECRET} {
        user {
            id {http.request.uri.query.user_id}
            name {http.request.header.X-User-Name}
        }
        roles "admin editor"
    }

    reverse_proxy backend:8080 {
        header_up X-JWT {http.jwt_signer.digest_str}
    }
}
```

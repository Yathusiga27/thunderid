# AuthZEN PEP demo with Cerbos

This sample runs ThunderID as a Policy Enforcement Point (PEP) during token authorization. ThunderID receives an OAuth request, builds an AuthZEN evaluation for the requested permission, sends it to Cerbos as the external Policy Decision Point (PDP), and uses Cerbos's decision to keep or remove the permission from the issued token.

The sample uses one travel-booking resource server:

```text
Resource server: https://api.example.com/travel-booking
Permissions:     booking:read, booking:create, booking:cancel, booking:upgrade
Cerbos:          http://localhost:3592
ThunderID:       https://localhost:8090
```

## 1. Cerbos external PDP

Cerbos directly exposes the AuthZEN endpoints used by the ThunderID PEP. No additional adapter service is required. Cerbos policies are stored outside ThunderID. For example:

```bash
export CERBOS_POLICY_DIR="/Users/cerbos-pdp/policies"
mkdir -p "$CERBOS_POLICY_DIR"
cp samples/pep-demo/cerbos/policies/travel-booking.yaml "$CERBOS_POLICY_DIR/"
```

The policy directory can be anywhere. The path above is only an example.

### Cerbos policy

The sample policy is [`cerbos/policies/travel-booking.yaml`](cerbos/policies/travel-booking.yaml). Its resource identifier must match the ThunderID resource server identifier:

```yaml
resource: "https://api.example.com/travel-booking"
```

The current ThunderID adapter sends group IDs as `subject.properties.groups`, so the policy checks that field:

```yaml
roles:
  - "*"
condition:
  match:
    expr: P.attr.groups.exists(group, group == "cerbos-travel-agent")
```

For example, the subject sent to Cerbos contains:

```json
{
  "subject": {
    "type": "user",
    "id": "travel-alice-customer",
    "properties": {
      "groups": ["cerbos-travel-agent"],
      "ouId": "default",
      "accountStatus": "active"
    }
  }
}
```

The adapter does not currently emit `cerbos.roles`. Do not change the policy to `roles: travel-agent` unless the adapter is changed to map group IDs to Cerbos roles.

### Start Cerbos locally

Cerbos is a separate local PDP process. It listens on HTTP port `3592` and watches the external policy directory.

Install the Cerbos binary using the [Cerbos installation guide](https://docs.cerbos.dev/cerbos/latest/installation/binary.html), then run this command from the repository root:

```bash
cerbos server \
  --set=server.httpListenAddr=:3592 \
  --set=storage.driver=disk \
  --set=storage.disk.directory="$CERBOS_POLICY_DIR" \
  --set=storage.disk.watchForChanges=true
```

Leave this process running in its terminal.

Verify the PDP before starting ThunderID:

```bash
curl -fsS http://localhost:3592/_cerbos/health
curl -fsS http://localhost:3592/.well-known/authzen-configuration
```

Docker is optional. If you prefer Docker instead of a local Cerbos binary:

```bash
docker run --rm --name thunderid-pep-cerbos \
  -v "$CERBOS_POLICY_DIR:/policies" \
  -p 3592:3592 \
  ghcr.io/cerbos/cerbos:latest server
```

## 2. Configure ThunderID to use Cerbos

Add this block in the ThunderID deployment configuration:

```yaml
authorization:
  external_authzen:
    enabled: true
    pdps:
      - name: cerbos
        endpoint: "http://localhost:3592/access/v1/evaluation"
        timeout_ms: 5000
        retry_count: 1
        subject_properties:
          - ouId
          - accountStatus
        resource_servers:
          - "https://api.example.com/travel-booking"
```

The important values are:

- `enabled: true` turns on external PDP routing.
- `endpoint` points to Cerbos's AuthZEN evaluation endpoint.
- `resource_servers` selects which resource server uses Cerbos. Other resource servers continue using the local RBAC engine.
- `subject_properties` controls which ThunderID user attributes are sent to Cerbos.

## 3. Start ThunderID

From the repository root:

```bash
make run
```

## 4. Import the demo resources

Import [`thunderid-resources.yaml`](thunderid-resources.yaml) into ThunderID. It creates:

- Travel customer and staff user types
- Alice, Bob, and Tina
- The `https://api.example.com/travel-booking` resource server
- The browser, upstream service, and token-exchange applications
- The `cerbos-travel-agent` group
- ThunderID RBAC roles and permission assignments used as the local fallback

## 5. Test with Postman

Create a Postman environment named `ThunderID AuthZEN PEP`:

| Variable | Value |
| --- | --- |
| `baseUrl` | `https://localhost:8090` |
| `resourceIdentifier` | `https://api.example.com/travel-booking` |
| `browserClientId` | `authzen-booking-browser` |
| `confidentialClientId` | `authzen-booking-client` |
| `exchangeClientId` | `authzen-booking-exchange-client` |
| `redirectUri` | `https://oauth.pstmn.io/v1/browser-callback` |
| `scope` | `booking:read booking:create booking:cancel booking:upgrade` |

Keep the client secrets in Postman secret variables. Their sample values are in [`thunderid.env`](thunderid.env).

### Client credentials token

Create a `POST` request to `{{baseUrl}}/oauth2/token` with **Basic Auth**:

- Username: `{{confidentialClientId}}`
- Password: `AUTHZEN_TEST_CLIENT_SECRET`

Use **x-www-form-urlencoded** body values:

```text
grant_type=client_credentials
resource={{resourceIdentifier}}
scope={{scope}}
```

The successful response should contain the permissions Cerbos allowed for `authzen-booking-client`. Save its `access_token` as `serviceAccessToken`.

### Token exchange

Create another `POST` request to `{{baseUrl}}/oauth2/token` with **Basic Auth**:

- Username: `{{exchangeClientId}}`
- Password: `AUTHZEN_EXCHANGE_CLIENT_SECRET`

Use this **x-www-form-urlencoded** body:

```text
grant_type=urn:ietf:params:oauth:grant-type:token-exchange
resource={{resourceIdentifier}}
scope=booking:create
subject_token={{serviceAccessToken}}
subject_token_type=urn:ietf:params:oauth:token-type:access_token
requested_token_type=urn:ietf:params:oauth:token-type:access_token
```

The response should contain only `booking:create`. Use the Postman variable `{{serviceAccessToken}}` rather than pasting an old JWT into `subject_token`.

### User token with authorization code and PKCE

Create an OAuth 2.0 token request in Postman with:

- Grant type: **Authorization Code (With PKCE)**
- Callback URL: `{{redirectUri}}`
- Auth URL: `{{baseUrl}}/oauth2/authorize`
- Access Token URL: `{{baseUrl}}/oauth2/token`
- Client ID: `{{browserClientId}}`
- Client secret: empty, because this is a public client
- Code challenge method: `SHA-256`
- Scope: for example, `booking:read booking:create`
- Authorization request parameter: `resource={{resourceIdentifier}}`

Sign in as Alice when Postman opens the browser. The token request must include the generated PKCE verifier and the browser client ID. The resulting token should contain only permissions allowed by Cerbos for Alice.

Inspect ThunderID logs for calls to `http://localhost:3592/access/v1/evaluations` and the returned decisions.

## What belongs to the feature

The backend PEP implementation does not depend on these sample files. The files provide a repeatable Cerbos demonstration:

- `thunderid-resources.yaml` defines the ThunderID identities, applications, resource server, roles, and groups.
- `cerbos/policies/travel-booking.yaml` defines the external Cerbos authorization policy.
- `thunderid.env` supplies local demo credentials for imports.

For the generic PEP implementation, the important runtime configuration is the `authorization.external_authzen` block in the deployment configuration.

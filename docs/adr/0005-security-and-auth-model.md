# 0005 — Security and Auth Model

**Status:** Accepted

## Context

The GCS handles safety-critical operations (arm/disarm, mode changes, mission upload) and may be accessed by multiple operators simultaneously over a network. Security must be present from day one but must not compromise MAVLink command latency — the primary operational constraint.

Two distinct concerns:
1. **Transport security** — encrypting and authenticating bytes on the wire
2. **Operator identity** — who is allowed to do what

---

## Decision

### Core principle: security is never in the MAVLink hot path

The MAVLink command path is:

```
React frontend → Connect (TLS — already established) → Go handler → UDP socket → ArduPilot
```

No per-packet crypto, no blocking auth checks, no audit writes occur in this path. All security work happens at connection time (TLS handshake, JWT validation) or asynchronously after the fact (audit log write to Redis).

---

### Transport security

**Connect (backend ↔ frontend):** TLS via HTTP/2. After the handshake, all Connect streaming and unary RPCs are encrypted at zero application cost. `TlsConfig` is set once per server startup.

**Redis:** TLS + AUTH password. Internal service boundary — not operator-facing. Localhost dev skips TLS; staging and production require it.

**MAVLink radio links:** MAVLink 2 signing (`MavlinkSigningConfig`) provides authentication and integrity on radio hops. It does not encrypt — encryption is the WireGuard layer's responsibility. Signing is configured once at link open time (`ConnectLinkRequest.security`) and has no per-packet application overhead; the radio firmware handles it.

**WireGuard for encrypted IP tunnels:** When MAVLink travels over IP (WiFi, cellular, internet relay), WireGuard provides encryption at the OS kernel level. The GCS application sees a plain UDP socket pointing at the WireGuard interface — zero application overhead. WireGuard configuration is managed at the deployment level (wg-quick, systemd-networkd, Docker network) and is not represented in the proto beyond the `uri` scheme on `ConnectLinkRequest`.

**WebRTC video:** DTLS-SRTP is mandatory by the WebRTC spec and handled automatically by the media server. No application configuration required.

**WFB-NG / wifibroadcast:** chacha20-poly1305 encryption is built into the wfb-ng link layer. Configured at the OS/driver level.

---

### Operator authentication: abstracted AuthProvider

The frontend authenticates directly against the configured auth provider (initially Supabase) and receives a JWT access token. The JWT is passed as `Authorization: Bearer <token>` on every Connect request header.

The backend validates the JWT through an `AuthProvider` interface:

```go
type AuthProvider interface {
    ValidateToken(ctx context.Context, token string) (*OperatorContext, error)
}
```

Implementations:
- `SupabaseAuthProvider` — validates against Supabase GoTrue (JWKS or shared secret)
- `OIDCAuthProvider` — any standards-compliant OIDC provider (Auth0, Keycloak, Clerk)
- `MockAuthProvider` — test doubles; returns a fixed `OperatorContext`
- `NoopAuthProvider` — SITL and local dev; skips validation, grants operator role

Swapping auth providers requires changing one configuration value — no service code changes.

**Login is not in the GCS proto.** The frontend uses the auth provider's own SDK (Supabase JS, Auth0 SDK, etc.) for login/signup flows. The GCS backend only validates tokens it receives. `AuthService.ValidateSession` exists for the frontend to verify its token is accepted by the GCS backend specifically (distinct from being valid with the auth provider).

---

### Authorization: role-based, enforced server-side

Three roles carried as JWT claims:

| Role | Permitted |
|---|---|
| `observer` | Subscribe to all streams, read state, watch video |
| `operator` | All of observer + issue commands, arm/disarm, change mode, upload missions |
| `admin` | All of operator + manage links, view audit log, configure link security |

Roles are extracted from the JWT by `AuthProvider.ValidateToken` and placed in `OperatorContext`. Backend middleware checks the role before each RPC handler runs. The client cannot assert or escalate its own role.

---

### Audit log: async, append-only

Every security-relevant action (command issued, arm/disarm, mode change, parameter set, link connected, auth failure) is written to a Redis Stream (`audit:events`) after the action completes. The write is a fire-and-forget goroutine — it never blocks the command path.

`AuditEvent` carries the `OperatorContext` (who), the event type (what), and a human-readable details string. No serialised request payloads — the log is for accountability and incident review, not replay.

The audit stream is retained in Redis with a configurable TTL (default 30 days). `SecurityService.WatchAuditLog` streams live events and can replay history on subscribe.

---

## Consequences

**Accepted costs:**
- Supabase (or any OIDC provider) is an external dependency for authentication. The `AuthProvider` abstraction means this can be replaced, but initial setup requires a Supabase project or equivalent.
- JWT expiry requires frontend token refresh logic. Expired tokens fail at the `AuthProvider` middleware level with a clear error.
- MAVLink signing requires the vehicle firmware to support MAVLink 2 and have the same key configured. `allow_unsigned` eases adoption.

**Expected benefits:**
- Zero auth overhead on the MAVLink command path — latency is unaffected
- Auth provider can be swapped (or stubbed for SITL) without touching service code
- Audit trail is durable, append-only, and queryable without stopping the system
- TLS and WireGuard are transport-layer — the application code is identical regardless of whether the link is encrypted
- Role enforcement is centralised in middleware — individual service handlers do not contain auth logic

# Loopback ingress rehearsal

`nginx.conf` is a standalone example that listens on `127.0.0.1:8081` and forwards to a loopback API at `127.0.0.1:8080`. It is disabled by default. Run `scripts/check-ingress.sh` to verify the same configuration against an isolated synthetic upstream. The Docker test publishes no ports.

The sample applies a single credential budget per TCP peer across registration, login, password changes, recovery, email verification and account deletion. The arrival rate is 12 requests per minute with a burst of five. The assisted analyst has its own budget with a burst of three. Rejections use 429 and Retry-After. Only the development dashboard origins receive CORS headers on ingress-generated rejections. A separate bound limits concurrent connections per peer.

The edge ignores and strips client-supplied forwarding headers. Use this example with direct peers. With a load balancer, configure only its trusted source ranges before deriving client identity from forwarding headers. Clients behind the same NAT share a budget. Limits are shared among workers on one ingress instance and are local to that instance. Review rate and capacity settings with representative traffic before rollout.

The local rehearsal leaves production TLS, authenticated proxy-to-API routing, multi-instance source limits, monitoring and deployment approval open. Keep the API listener inaccessible to clients who should use the edge.

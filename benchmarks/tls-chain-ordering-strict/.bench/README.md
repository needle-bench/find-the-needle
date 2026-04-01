# tls-chain-ordering-strict

## Difficulty
Extreme

## Source
Community-submitted

## Environment
Go 1.22, Alpine Linux, OpenSSL

## The bug
The PEM bundle `/certs/chain.pem` is assembled during Docker build with:
```
cat root.crt intermediate.crt leaf.crt > chain.pem
```
Go's `tls.X509KeyPair` finds the leaf cert (matching the private key) and places it at `Certificate[0]`. The remaining certificates are appended in PEM file order: root CA first, intermediate CA second. The server sends the chain as [leaf, root, intermediate].

RFC 5246 Section 7.4.2 requires: leaf, then each certificate that directly certifies the preceding one. So the correct order is [leaf, intermediate, root]. Lenient TLS clients (curl, Go, browsers) build a trust graph and connect regardless. Strict validators process certs sequentially and reject the connection because cert[1] (root) did not issue cert[0] (leaf).

## Why Extreme
1. **Test 1 passes** — the lenient curl client connects without errors, making the server appear correct.
2. The red herring comment about certificate expiry in the loader looks suspicious but is actually correct (the standard library handles expiry validation).
3. The agent must understand RFC 5246 chain ordering semantics and that `X509KeyPair` preserves PEM file order for non-leaf certificates.
4. The bug spans two files: the Dockerfile (PEM concatenation order) and the Go loader (no post-load chain sorting).
5. Two viable fix strategies: fix the `cat` order in the Dockerfile, or add chain-sorting logic in the Go loader. The loader fix is more robust.

## Expected fix
**Option A (loader fix, preferred):** In `app/tlssetup/loader.go`, after calling `X509KeyPair`, sort `tlsCert.Certificate[1:]` by Subject/Issuer relationships. Find the cert whose Subject matches the leaf's Issuer, place it at index 1, then find the cert whose Subject matches that cert's Issuer, place it at index 2, etc.

**Option B (Dockerfile fix, simpler):** Change the `cat` command in the Dockerfile from `cat root.crt intermediate.crt leaf.crt` to `cat leaf.crt intermediate.crt root.crt`. This puts the PEM in correct order, so `X509KeyPair` preserves it.

Both fixes are accepted by the test suite.

## Pinned at
Anonymized snapshot, original repo not disclosed

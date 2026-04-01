# tls-chain-ordering-strict

## Project

A Go HTTPS server that loads a 3-level TLS certificate chain (Root CA -> Intermediate CA -> Leaf) from a PEM bundle and serves traffic on port 8443. The certificate chain is generated at Docker build time using OpenSSL with real RSA keys. The cert loader uses `tls.X509KeyPair` to match the private key to the leaf certificate.

## Symptoms

A lenient TLS client (curl with `--cacert`, Go's `net/http`, most browsers) connects to the server without errors. The `/health` endpoint returns `200 OK`. Everything appears to work.

However, a strict chain-order check per RFC 5246 Section 7.4.2 reveals that the certificates after the leaf are in the wrong order. The server sends: leaf, root CA, intermediate CA. The correct order is: leaf, intermediate CA, root CA. Each certificate should be followed by its issuer, forming an ascending trust path.

Strict TLS validators (BoringSSL, certain Java TLS stacks, embedded devices) process the chain sequentially and fail because cert[1] (root CA) did not issue cert[0] (leaf) — the intermediate CA did.

## Bug description

The PEM bundle at `/certs/chain.pem` is assembled during Docker build with `cat root.crt intermediate.crt leaf.crt`. Go's `tls.X509KeyPair` correctly identifies the leaf certificate (the one matching the private key) and places it at `Certificate[0]`. However, the remaining chain certificates are appended in their original PEM file order: root, then intermediate. This produces the chain [leaf, root, intermediate] instead of the correct [leaf, intermediate, root].

The fix: after loading the certificate via `X509KeyPair`, sort the chain certificates (indices 1..n) so that each cert[i] is issued by cert[i+1], forming a proper issuer chain per RFC 5246.

## Difficulty

Extreme

## Expected turns

15-25

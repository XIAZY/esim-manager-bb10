Root certificates for SM-DP+ and SM-DS TLS, in addition to the public web
roots. `osmocom-ci-bundle.pem` is Osmocom's bundle of the production GSMA
SGP.22 certificate issuers (CIs), unchanged:
https://euicc-manual.osmocom.org/docs/pki/ci/bundle.pem

| CI | Subject key ID | Valid until |
| --- | --- | --- |
| OISTE GSMA CI G1 | 4C27967AD20C14B391E9601E41E604AD57C0222F | 2059-01-07 |
| GSM Association - RSP2 Root CI1 | 81370F5125D0B1D408D4C3B232E6D25E795BEBFB | 2052-02-21 |
| Entrust eSIM Certification Authority | 16704B7F351E3607F18C4B70005C3A003DFD414A | 2051-10-16 |
| MC4 OT ROOT CI v1 (Oberthur, now IDEMIA) | CD6E60B72D7A063CBC84625B91E80FE30406E13E | 2046-11-08 |
| SubMan V4.2 CI Google Pixel (G+D) | B60F0B897FD630B88CFED6161F8EA808C5382AC3 | 2027-05-10 |
| SubMan V4.2 CI (G+D) | EA53ADEF3329B9509AFD755FD448B82DC1BCECFF | expired 2026-08-12 |

The GSMA root's SHA-256 fingerprint,
5E3E91FD454327C3AF5D32A7A73BBC59FE43AA7D85FD32D5DB44423F80A56BB3, and the
Pixel SubMan CI's, 2CE2D6787F3B6411B89E675E7EB3E87F88179BF5B08A6EC78D95A6167700C78B,
also match the roots ChromiumOS's eSIM manager ships (platform2/hermes/certs/prod).
Expired roots are ignored when verifying. To update, download the bundle again.

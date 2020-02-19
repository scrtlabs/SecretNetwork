Import the Enigma public GPG key:

```bash
gpg2 --recv-keys 91831DE812C6415123AFAA7B420BF1CB005FBCE6
```

Check release hashes:

```bash
sha256sum --check SHA256SUMS.asc
```

Verify the hashes are singed by Enigma:

```bash
gpg2 --verify SHA256SUMS.asc
```

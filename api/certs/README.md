# Local JWT signing keys

The repository never ships a reusable private key. Create a local RSA key pair
from the `api` directory before starting the API:

```sh
umask 077
openssl genpkey -algorithm RSA \
  -pkeyopt rsa_keygen_bits:3072 \
  -out certs/private_key.pem
openssl pkey -in certs/private_key.pem \
  -pubout -out certs/public_key.pem
chmod 600 certs/private_key.pem
chmod 644 certs/public_key.pem
```

Both generated files are ignored by Git. Production keys come from the
operator-owned secret files mounted by the deployment topology; never copy a
development key into a release environment.

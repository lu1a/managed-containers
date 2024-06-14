# Core Service

This assumes that you already have a kubernetes cluster up and running according to [../kube-setup](../kube-setup/README.md), and the kubeconfig for that cluster is under your `.kube` directory such as `fi-hel1.conf` (your cluster name is your _zone_ name now).

Additionally, we assume you have a postgres setup somewhere, according to [../postgres-setup](../postgres-setup/README.md)

First load up a .env file based on the .env.example.

Run migrations in your postgres database using sql-migrate.

```bash
cd core-service
```

```bash
export $(grep -v '^#' .env | xargs) && sql-migrate up
```

```bash
go run main.go
```

Optional:

```bash
./tailwindcss -o ./frontend/static/styles.css --watch
```

Note to self: don't forget to run `./tailwindcss -o ./frontend/static/styles.css --minify` to "build" the css file.

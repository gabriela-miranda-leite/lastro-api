# lastro-api

Backend do Lastro — controle financeiro pessoal e de casal.

**API em produção:** https://lastro-api-8927.onrender.com

> O serviço usa o free tier do Render. Se estiver inativo, a primeira requisição pode demorar ~30s (cold start).

---

## Stack

- Go 1.26 + Gin
- PostgreSQL (Neon serverless)
- golang-migrate (migrations automáticas no startup)
- JWT (access 1h + refresh 30d com rotação)
- testcontainers-go (testes de integração)

---

## Rodar local

### Pré-requisitos

- Go 1.26+
- Docker (para o banco local nos testes)
- [golang-migrate CLI](https://github.com/golang-migrate/migrate) (opcional, migrations rodam no startup)

### 1. Variáveis de ambiente

```bash
cp .env.example .env
```

Preencher `.env`:

| Variável | Descrição |
|---|---|
| `DATABASE_URL` | String de conexão PostgreSQL (non-pooled para migrations) |
| `JWT_SECRET` | Segredo HS256, mínimo 32 chars (`openssl rand -hex 32`) |
| `ENV` | `development` ou `production` |
| `PORT` | Porta da API (default: `8080`) |

### 2. Subir o banco local

```bash
docker run -d \
  --name lastro-db \
  -e POSTGRES_USER=lastro \
  -e POSTGRES_PASSWORD=lastro \
  -e POSTGRES_DB=lastro \
  -p 5432:5432 \
  postgres:16-alpine
```

```
DATABASE_URL=postgres://lastro:lastro@localhost:5432/lastro?sslmode=disable
```

### 3. Rodar a API

```bash
go run ./cmd/api/main.go
```

As migrations são aplicadas automaticamente no startup.

---

## Comandos úteis

```bash
go build ./...            # compilar
go test ./...             # todos os testes (requer Docker para integração)
golangci-lint run         # lint
make migrate-up           # aplicar migrations manualmente
make migrate-down         # reverter última migration
swag init -g cmd/api/main.go -o docs  # gerar docs Swagger
```

---

## Estrutura de pastas

```
lastro-api/
  cmd/api/main.go        ← entrypoint
  internal/
    auth/                ← autenticação JWT
    user/                ← perfil, export, delete (LGPD)
    transaction/         ← CRUD de transações
    recurring/           ← despesas fixas + job mensal
    category/            ← categorias de transações
    goal/                ← metas financeiras
    couple/              ← vinculação de casal
    dashboard/           ← resumo financeiro mensal
    audit/               ← log de ações imutável
  db/
    migrations/          ← arquivos .sql versionados
    queries/             ← queries .sql para o sqlc
  docs/                  ← Swagger gerado pelo swag
```

---

## Coleção Insomnia

Importe `insomnia.json` na raiz do repositório (**File → Import**).

Ambientes disponíveis: `Lastro — Production` e `Lastro — Local`.  
Após login, copie `access_token` e `refresh_token` para as variáveis de ambiente.

---

## Documentação da API

Disponível em `/swagger/index.html` quando rodando local.

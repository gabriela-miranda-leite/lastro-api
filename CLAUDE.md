# lastro-api

Backend da aplicação Lastro — controle financeiro pessoal e de casal.

## Stack
- Go 1.22+
- Gin (router e middleware HTTP)
- PostgreSQL (valores monetários sempre como `NUMERIC(12,2)`, nunca FLOAT)
- sqlc (queries type-safe)
- golang-migrate (versionamento do schema)
- testify + testcontainers-go (testes)
- golangci-lint (lint)
- swaggo/swag (documentação Swagger)

## Module path
```
github.com/gabriela-miranda-leite/lastro-api
```

## Estrutura de pastas
```
lastro-api/
  cmd/api/main.go           ← entrypoint, bootstrap do servidor
  internal/
    auth/
      handler.go            ← HTTP: parse, validação, resposta
      service.go            ← lógica de negócio (sem HTTP, sem SQL)
      repository.go         ← queries SQL (sem lógica de negócio)
      types.go              ← structs de request/response
    user/
    transaction/
    couple/
    category/
    goal/
    recurring/
    dashboard/
    audit/
  db/
    migrations/             ← arquivos .sql numerados (golang-migrate)
    queries/                ← queries .sql para o sqlc
  config/
    config.go               ← lê variáveis de ambiente, falha rápido se faltar
  docs/                     ← gerado pelo swag init (não editar manualmente)
  Dockerfile
  .golangci.yml
  .env.example
  Makefile
```

## Padrão de código

### Handlers (Gin)
```go
func (h *Handler) CreateTransaction(c *gin.Context) {
    var input CreateTransactionInput
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
        return
    }
    userID := c.GetString("user_id")
    result, err := h.service.CreateTransaction(c.Request.Context(), userID, input)
    if err != nil {
        // mapear erros de domínio para HTTP aqui
        return
    }
    c.JSON(http.StatusCreated, result)
}
```

### Services
```go
func (s *Service) CreateTransaction(ctx context.Context, userID string, input CreateTransactionInput) (Transaction, error)
```
- Nunca importar `net/http` ou `gin` em service files
- Sempre `context.Context` como primeiro parâmetro

### Repositories
```go
func (r *Repository) InsertTransaction(ctx context.Context, tx Transaction) (Transaction, error)
```
- Nunca importar `net/http` ou `gin` em repository files
- Sempre queries parametrizadas — nunca concatenar input do usuário em SQL

### Erros
```go
// sempre wrappear com contexto
return fmt.Errorf("service.CreateTransaction: %w", err)

// sentinels para casos conhecidos
var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrConflict = errors.New("conflict")
```

## Variáveis de ambiente
| Variável | Descrição | Obrigatória |
|---|---|---|
| `DATABASE_URL` | String de conexão PostgreSQL | sim |
| `JWT_SECRET` | Segredo para assinar tokens JWT | sim |
| `PORT` | Porta da API (default: 8080) | não |
| `ENV` | `development` ou `production` | sim |

A aplicação deve falhar no startup se qualquer variável obrigatória estiver ausente.

## Comandos úteis
```bash
go build ./...          # compilar
go test ./...           # rodar todos os testes
golangci-lint run       # lint
make migrate-up         # aplicar migrations
make migrate-down       # reverter última migration
swag init -g cmd/api/main.go -o docs  # gerar docs Swagger
```

## Regras de negócio importantes
- Valores monetários: sempre `NUMERIC(12,2)` no banco, nunca FLOAT
- Soft delete: `deleted_at TIMESTAMPTZ` — nunca hard delete em transações e usuários
- Idempotência: transações usam `client_id` (UUID gerado no device) para evitar duplicatas
- audit_log: sem FK em `user_id` (preservado após exclusão de conta); imutável via API
- JWT: access token expira em 1h; refresh token em 30d com rotação a cada uso
- LGPD: usuário pode exportar e excluir seus dados; purge definitivo após 30 dias do soft delete

## Proibido
- `panic()` fora do `main.go`
- Estado global mutável
- `interface{}` ou `any` em assinaturas públicas
- Ignorar erros com `_`
- `time.Sleep` em código de produção
- Segredos hardcoded

## Projeto Jira
ANCHOR — https://gabrielamiranda1110-1789940258073.atlassian.net/browse/ANCHOR
Todas as tasks de backend têm label `back`.
Commits devem referenciar a task: `feat(auth): implement JWT middleware\n\nRefs: ANCHOR-20`

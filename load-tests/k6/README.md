# Load Tests — Lastro API

Testes de carga com [k6](https://k6.io/) para os três cenários de LASTRO-55.

## Pré-requisitos

```bash
# macOS
brew install k6

# Linux
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update && sudo apt-get install k6
```

## Executando os testes

Suba a API e o banco localmente (`make run` com `DATABASE_URL` configurado) antes de rodar qualquer teste.

### Via Makefile (recomendado)

```bash
make load-test           # roda os três cenários em sequência
make load-test-dashboard
make load-test-transactions
make load-test-idempotency
```

### Diretamente com k6

```bash
# 50 VUs — GET /dashboard
k6 run load-tests/k6/scripts/dashboard.js \
  --out json=load-tests/k6/reports/dashboard.json

# 20 VUs — POST /transactions
k6 run load-tests/k6/scripts/transactions.js \
  --out json=load-tests/k6/reports/transactions.json

# Idempotência — 10 threads, mesmo client_id
k6 run load-tests/k6/scripts/idempotency.js \
  --out json=load-tests/k6/reports/idempotency.json
```

Altere a URL base com a variável de ambiente `BASE_URL`:

```bash
k6 run load-tests/k6/scripts/dashboard.js \
  -e BASE_URL=https://api.staging.lastro.app \
  --out json=load-tests/k6/reports/dashboard.json
```

## Métricas esperadas

| Métrica | Limiar |
|---|---|
| P95 de latência (`http_req_duration`) | < 500 ms |
| Taxa de erros (`http_req_failed`) | 0% (zero 5xx) |
| Duplicatas no teste de idempotência | 0 |

Os thresholds estão definidos em cada script — o k6 retorna exit code 1 se algum falhar.

## Verificando idempotência

Após rodar `idempotency.js`, o script imprime a query de verificação no console.
Execute no banco:

```sql
SELECT COUNT(*)
FROM transactions
WHERE client_id = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee'
  AND deleted_at IS NULL;
-- deve retornar 1
```

## Relatórios

Os arquivos JSON gerados em `reports/` podem ser convertidos para HTML com
[k6-reporter](https://github.com/benc-uk/k6-reporter):

```bash
npx k6-reporter load-tests/k6/reports/dashboard.json
```

Os arquivos `.json` de relatório **não são versionados** (ver `.gitignore`).

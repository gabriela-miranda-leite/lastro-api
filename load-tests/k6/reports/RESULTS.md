# Load Test Results — 2026-10-18

Executado contra `http://localhost:8080` (ambiente de desenvolvimento).

## dashboard.js — 50 VUs × 30s

| Métrica | Valor |
|---|---|
| P95 latência | **13.95 ms** ✓ (< 500 ms) |
| Taxa de erro 5xx | **0.00%** ✓ |
| Requisições totais | 14 415 |
| Throughput | 448 req/s |

## transactions.js — 20 VUs × 30s

| Métrica | Valor |
|---|---|
| P95 latência | **13.50 ms** ✓ (< 500 ms) |
| Taxa de erro 5xx | **0.00%** ✓ |
| Requisições totais | 11 015 |
| Throughput | 343 req/s |

## idempotency.js — 10 threads simultâneas

| Métrica | Valor |
|---|---|
| P95 latência | < 500 ms ✓ |
| Erros 5xx | 0 ✓ |
| Duplicatas no banco | **0** ✓ (1 linha para 10 tentativas) |

Verificação SQL:
```sql
SELECT COUNT(*) FROM transactions
WHERE client_id = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee'
  AND deleted_at IS NULL;
-- resultado: 1
```

## Conclusão

Todos os thresholds atendidos. A API suporta uso concorrente sem degradação nem duplicatas.

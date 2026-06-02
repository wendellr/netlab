# BGP Lab 1 — Guia de Deploy (Professor)

## Estrutura de arquivos

```
bgp-lab1/
├── docker-compose.yml       ← sobe roteadores FRR + ttyd
├── nginx-bgp-lab1.conf      ← proxy reverso para os terminais web
├── roteiro-aluno.md         ← roteiro para os alunos
└── frr/
    ├── r1/
    │   ├── frr.conf         ← config BGP base do R1 (AS1)
    │   └── daemons          ← habilita zebra + bgpd
    ├── r2/  (idem)
    ├── r3/  (idem)
    └── r4/  (idem)
```

---

## Deploy no servidor (com Portainer ou direto via SSH)

### 1. Copie os arquivos para o servidor

```bash
scp -r bgp-lab1/ usuario@servidor:/opt/labs/bgp-lab1
```

### 2. Suba o lab via docker compose

```bash
cd /opt/labs/bgp-lab1
docker compose up -d
```

No Portainer: Stacks → Add Stack → Upload → selecione o `docker-compose.yml`.
Coloque o caminho dos volumes frr/ corretamente (caminho absoluto no servidor).

### 3. Ajuste o Nginx

Edite `nginx-bgp-lab1.conf` e altere `server_name` para o seu domínio.
Copie para o servidor:

```bash
cp nginx-bgp-lab1.conf /etc/nginx/conf.d/
nginx -t && systemctl reload nginx
```

### 4. Verifique os containers

```bash
docker compose ps
# Esperado: r1, r2, r3, r4, ttyd-r1..r4 todos "running"
```

Teste manual de um terminal:
```bash
curl http://localhost:7681   # deve retornar HTML do ttyd
```

---

## Verificação rápida das sessões BGP

```bash
docker exec bgp_lab1_r1 vtysh -c "show ip bgp summary"
docker exec bgp_lab1_r4 vtysh -c "show ip bgp"
```

Sessões devem aparecer como `Established` após ~30s.

---

## Reset do lab (para nova turma ou nova tentativa)

```bash
docker compose down
docker compose up -d
```

Os arquivos `frr.conf` são read-only montados como volume — o container
reinicia sempre com a config base. Alterações feitas pelos alunos via
vtysh são **perdidas no restart** (comportamento intencional para labs).

---

## Notas sobre FRR vs IOS

| IOS | FRR (vtysh) | Obs |
|-----|-------------|-----|
| `show ip bgp` | `show ip bgp` | idêntico |
| `show ip bgp summary` | `show ip bgp summary` | idêntico |
| `neighbor X ebgp-multihop 3` | `neighbor X ebgp-multihop 3` | idêntico |
| `set as-path prepend` | `set as-path prepend` | idêntico |
| `bgp always-compare-med` | `bgp always-compare-med` | idêntico |
| `clear ip bgp X soft` | `clear ip bgp X soft` | idêntico |
| `interface Loopback0` | `interface lo` | diferente — usar `lo` |
| `update-source Loopback0` | `update-source lo` | idem |

---

## Portas utilizadas

| Serviço | Porta (host) |
|---------|-------------|
| ttyd R1 | 7681 |
| ttyd R2 | 7682 |
| ttyd R3 | 7683 |
| ttyd R4 | 7684 |

Nginx faz o proxy de `/bgp/r1..r4` para essas portas.
As portas **não precisam estar abertas no firewall** — apenas a 80/443 do Nginx.

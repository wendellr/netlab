# BGP Lab 1 — AS-Path Prepend e MED
### Disciplina: Roteamento IP / Redes de Alta Velocidade — IFCE

---

## Guia rapido do aluno

1. Abra os terminais web (cada roteador abre direto no vtysh):
   - http://lab.seudominio.com.br/bgp/r1
   - http://lab.seudominio.com.br/bgp/r2
   - http://lab.seudominio.com.br/bgp/r3
   - http://lab.seudominio.com.br/bgp/r4

2. Parte 1: valide a configuracao base em todos os roteadores:

```
show ip bgp summary
show ip bgp
show ip route bgp
```

3. Parte 2: execute o AS-Path prepend em R1 e confira em R4:

```
conf t
route-map PREP permit 10
 set as-path prepend 1 1 1 1
exit
router bgp 1
 neighbor 4.4.4.4 route-map PREP out
end
clear ip bgp 4.4.4.4 soft out
```

4. Parte 3: aplique MED em R2 e R4, depois ajuste o criterio em R3:

R4:
```
conf t
route-map MED permit 10
 set metric 4
exit
router bgp 4
 neighbor 3.3.3.3 route-map MED out
end
clear ip bgp 3.3.3.3 soft out
```

R2:
```
conf t
route-map MED permit 10
 set metric 2
exit
router bgp 2
 neighbor 3.3.3.3 route-map MED out
end
clear ip bgp 3.3.3.3 soft out
```

R3:
```
conf t
router bgp 3
 bgp always-compare-med
 bgp bestpath as-path ignore
end
clear ip bgp * soft
```

5. Para verificar, use sempre:

```
show ip bgp
show ip bgp <prefixo>
```

## Topologia

```
                         AS1           AS2           AS3           AS4
                      +-----+       +-----+       +-----+       +-----+
                      | R1  |-------| R2  |-------| R3  |-------| R4  |
                      |1.1.1.1|     |2.2.2.2|     |3.3.3.3|     |4.4.4.4|
                      +--+--+       +-----+       +-----+       +--+--+
                           |                                             |
                           |        LAN compartilhada 150.1.1.0/24        |
                           +---------------------------------------------+
                                    (R1 = 150.1.1.1, R4 = 150.1.1.4)

Links de transito (roteadores adjacentes):
- R1-R2: 10.0.0.0/29
- R2-R3: 10.0.0.8/29
- R3-R4: 10.0.0.16/29 e 10.0.0.24/29 (dois enlaces)
```

**Peering BGP usa loopbacks** (ebgp-multihop 3). R3-R4 têm dois links físicos.

Prefixos anunciados:
- R1/R4 → `150.1.1.0/24`
- R2 → `150.2.2.0/24`
- R3 → `150.3.3.0/24`

---

## Acesso aos roteadores

Abra no navegador:

| Roteador | URL |
|----------|-----|
| R1 — AS1 | http://lab.seudominio.com.br/bgp/r1 |
| R2 — AS2 | http://lab.seudominio.com.br/bgp/r2 |
| R3 — AS3 | http://lab.seudominio.com.br/bgp/r3 |
| R4 — AS4 | http://lab.seudominio.com.br/bgp/r4 |

O terminal abre direto no **vtysh** (CLI equivalente ao IOS).

---

## Parte 1 — Verificação da configuração base

Nos quatro roteadores, verifique:

```
R1# show ip bgp summary
R1# show ip bgp
R1# show ip route bgp
```

**Questão 1:** No `show ip bgp` de R4, qual é o next-hop preferido para `150.2.2.0/24`?
Por quê R4 escolhe esse caminho? Qual atributo BGP determina essa escolha?

**Questão 2:** O que significa o `?` na coluna Origin da tabela BGP?
Qual seria o símbolo para uma rota originada com `network` statement?

---

## Parte 2 — AS-Path Prepend

**Objetivo:** Forçar R4 a preferir o caminho via R3 para **todos** os prefixos de AS1.

**Contexto:** R4 tem dois caminhos para `150.2.2.0/24`:
- Via R1: AS_PATH = `1 2`
- Via R3: AS_PATH = `3 2`

R4 prefere via R1 pelo menor Router-ID. Queremos mudar isso sem alterar a topologia física.

**Ação — configure em R1:**

```
R1# conf t
R1(config)# route-map PREP permit 10
R1(config-route-map)# set as-path prepend 1 1 1 1
R1(config-route-map)# exit
R1(config)# router bgp 1
R1(config-router)# neighbor 4.4.4.4 route-map PREP out
R1(config-router)# end
R1# clear ip bgp 4.4.4.4 soft out
```

**Verificação em R4:**

```
R4# show ip bgp
R4# show ip bgp 150.2.2.0/24
```

**Questão 3:** Como ficou o AS_PATH do caminho via R1 após o prepend?
Qual passo da **BGP Decision Process** foi afetado?

**Questão 4:** O prepend foi aplicado apenas para R4 ou para todos os peers de R1?
Por quê? O que seria diferente se aplicássemos `neighbor 2.2.2.2 route-map PREP out`?

---

## Parte 3 — MED (Multi-Exit Discriminator)

**Objetivo:** Configurar R2 (MED=2) e R4 (MED=4) para que R3 prefira o caminho via R2.

**Ação — configure em R4:**

```
R4# conf t
R4(config)# route-map MED permit 10
R4(config-route-map)# set metric 4
R4(config-route-map)# exit
R4(config)# router bgp 4
R4(config-router)# neighbor 3.3.3.3 route-map MED out
R4(config-router)# end
R4# clear ip bgp 3.3.3.3 soft out
```

**Ação — configure em R2:**

```
R2# conf t
R2(config)# route-map MED permit 10
R2(config-route-map)# set metric 2
R2(config-route-map)# exit
R2(config)# router bgp 2
R2(config-router)# neighbor 3.3.3.3 route-map MED out
R2(config-router)# end
R2# clear ip bgp 3.3.3.3 soft out
```

**Verificação em R3:**

```
R3# show ip bgp
R3# show ip bgp 150.1.1.0/24
```

**Questão 5:** R3 mudou de comportamento? Por quê o MED ainda não funciona?
Identifique os **dois problemas** descritos no lab.

**Ação — corrija em R3:**

```
R3# conf t
R3(config)# router bgp 3
R3(config-router)# bgp always-compare-med
R3(config-router)# bgp bestpath as-path ignore
R3(config-router)# end
R3# clear ip bgp * soft
```

**Questão 6:** Agora R3 prefere o caminho via R2? Confirme com `show ip bgp`.

**Questão 7:** Explique a diferença entre os dois comandos aplicados em R3.
Em qual cenário do mundo real seria perigoso usar `bgp bestpath as-path ignore`?

---

## Resumo dos atributos trabalhados

| Atributo | Escopo | Quem define | Quem usa | Menor/Maior = melhor? |
|----------|--------|-------------|----------|----------------------|
| AS_PATH length | eBGP | Qualquer AS | Receptor | Menor |
| MED | eBGP | AS de origem | AS vizinho | Menor |
| LOCAL_PREF | iBGP | AS local | Roteadores do mesmo AS | Maior |

---

## Comandos úteis de referência

```
show ip bgp                        → tabela BGP completa
show ip bgp summary                → estado dos peers
show ip bgp <prefixo>              → detalhe de um prefixo
show ip bgp neighbors <ip>         → detalhe de um peer
show route-map                     → route-maps configurados
clear ip bgp <ip> soft out         → reseta sessão sem derrubar
clear ip bgp * soft                → reseta todas as sessões
```

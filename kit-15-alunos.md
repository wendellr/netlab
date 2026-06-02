# Kit local para 15 alunos

Este kit sobe 15 copias do lab na mesma maquina, cada uma com portas proprias.

## Preparacao

1. Garanta que os containers do lab base estao parados:

```
docker compose down
```

2. Dê permissao de execucao aos scripts:

```
chmod +x tools/start-class.sh tools/stop-class.sh
```

## Subir 15 copias

```
./tools/start-class.sh
```

Isso cria:
- 15 stacks (aluno01 .. aluno15)
- Portas em blocos de 10 por aluno

Por padrao:
- aluno01: 7681-7684
- aluno02: 7691-7694
- aluno03: 7701-7704
- ...

A saida do script imprime as URLs de cada aluno.

## Parar todas as copias

```
./tools/stop-class.sh
```

## Parametros (opcional)

```
./tools/start-class.sh <quantidade> <porta_base> <salto_porta> <pasta_env>
```

Exemplo para 10 alunos com base 8001:

```
./tools/start-class.sh 10 8001 10 class-env
```

## Dica

Se faltar porta, aumente o salto (ex.: 20) ou escolha outra base.

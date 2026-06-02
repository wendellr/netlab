# Netlab - BGP Lab 1

Laboratorio de BGP (AS-Path Prepend e MED) com FRR, terminais web (ttyd) e pagina de onboarding. Projeto da disciplina de Roteamento IP / Redes de Alta Velocidade - IFCE.

## O que tem aqui

- Lab BGP com 4 roteadores FRR (R1..R4).
- Terminais web via ttyd.
- Pagina de onboarding em HTML com roteiro e links.
- Kit para 15 alunos com stacks isoladas e portas dedicadas.

## Subir localmente (nginx no docker)

1. Suba os containers:

```
docker compose --profile nginx up -d
```

2. Acesse a pagina:

- http://localhost/

Rotas para os terminais:

- http://localhost/r1/
- http://localhost/r2/
- http://localhost/r3/
- http://localhost/r4/

## Somente ttyd (sem nginx)

Se quiser acessar direto pelas portas:

- http://localhost:7681
- http://localhost:7682
- http://localhost:7683
- http://localhost:7684

## Roteiro do aluno

O roteiro completo esta em [roteiro-aluno.md](roteiro-aluno.md).

## Onboarding

A pagina de onboarding esta em [onboarding.html](onboarding.html) e pode ser servida pelo nginx do compose.

## Kit para 15 alunos

Use o script para subir 15 copias do lab:

```
chmod +x tools/start-class.sh tools/stop-class.sh
./tools/start-class.sh
```

Para parar tudo:

```
./tools/stop-class.sh
```

Detalhes em [kit-15-alunos.md](kit-15-alunos.md).

## Provisionamento sob demanda (Portainer)

Existe um provisionador em Go que cria labs sob demanda e derruba em 2h. Ele usa Redis e acessa o Docker via socket.

### Como habilitar

- Suba a stack principal com perfis:

```
COMPOSE_PROFILES=nginx,provisioner
```

- Variaveis principais:

```
PROVISIONER_PORT=9002
REDIS_ADDR=127.0.0.1:6379
COMPOSE_DIR=/data/compose/<STACK_ID>
PUBLIC_BASE_URL=https://netlab.ioda.com.br
```

O provisionador monta os links em paths (`/alunoXX/r1`) e atualiza o Nginx automaticamente.

## Arquivos principais

- [docker-compose.yml](docker-compose.yml)
- [nginx/nginx.conf](nginx/nginx.conf)
- [onboarding.html](onboarding.html)
- [roteiro-aluno.md](roteiro-aluno.md)
- [frr](frr)

## Observacoes

- O nginx interno serve a pagina e faz proxy para os ttyd.
- O acesso remoto deve ser ajustado para o dominio final (ex.: https://netlab.ioda.com.br).

## Problemas comuns

- **Porta ocupada:** pare o serviço que esta usando a porta 80 ou use outro bind no compose.
- **Terminal web nao responde:** recarregue a pagina, clique no terminal e confirme o link.
- **Links dos alunos:** use o script [tools/start-class.sh](tools/start-class.sh) para subir todas as copias.
